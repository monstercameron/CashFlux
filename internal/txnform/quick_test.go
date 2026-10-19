// SPDX-License-Identifier: MIT

package txnform

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountselect"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/learntally"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/smartcatview"
)

func quickTestSource() QuickSource {
	return QuickSource{
		Accounts:   []domain.Account{{ID: "usd", Name: "Checking", Currency: "USD", Scope: domain.ScopeIndividual, OwnerID: "owner"}, {ID: "jpy", Name: "Cash", Currency: "JPY"}},
		Categories: []domain.Category{{ID: "out", Name: "Food", Kind: domain.KindExpense}, {ID: "in", Name: "Salary", Kind: domain.KindIncome}},
		Members:    []domain.Member{{ID: "owner", Name: "Owner", Color: "red"}}, DefaultAccount: "usd",
		Now: time.Date(2024, 2, 29, 23, 59, 0, 0, time.FixedZone("west", -8*3600)),
	}
}

func TestQuickValidationAndBuild(t *testing.T) {
	s := quickTestSource()
	for _, tc := range []struct {
		name, account, raw, direction, payee, desc, field string
		minor                                             int64
	}{
		{"expense", "usd", "12.50", "Expense", " Market ", "", "", -1250},
		{"income", "usd", "12.50", "Income", "", " Salary ", "", 1250},
		{"minus wins", "usd", "-1", "Income", "Market", "", "", -100},
		{"zero", "usd", "0", "Expense", "M", "", "txn-add-amount", 0},
		{"negative zero", "usd", "-0", "Expense", "M", "", "txn-add-amount", 0},
		{"overflow", "usd", "92233720368547758.08", "Income", "M", "", "txn-add-amount", 0},
		{"minimum signed amount", "usd", "-92233720368547758.08", "Expense", "M", "", "", math.MinInt64},
		{"huge", "usd", strings.Repeat("9", 10000), "Expense", "M", "", "txn-add-amount", 0},
		{"empty", "usd", "", "Expense", "M", "", "txn-add-amount", 0},
		{"expression requires blur", "usd", "2*3", "Expense", "M", "", "txn-add-amount", 0},
		{"missing account first", "missing", "oops", "Expense", "", "", "txn-add-account", 0},
		{"missing payee", "usd", "1", "Expense", " \t", "\n", "txn-add-payee", 0},
		{"zero decimals", "jpy", "123", "Income", "M", "", "", 123},
		{"zero decimals rejects fraction", "jpy", "1.25", "Expense", "M", "", "txn-add-amount", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := NewQuickDraft("")
			d.Amount, d.Direction, d.Payee, d.Desc = tc.raw, tc.direction, tc.payee, tc.desc
			d.Date, d.Tags, d.Note = "2024-02-29", " a, b, , c ", " note "
			d.Cleared, d.Exclude = true, true
			field, key := QuickInvalidField(s.Accounts, tc.account, tc.raw, tc.direction, tc.payee, tc.desc)
			if field != tc.field || (key == "") != (field == "") {
				t.Fatalf("validation = %q %q", field, key)
			}
			txn, err := BuildQuick(d, s.Accounts, tc.account, "", "id", s.Now)
			if (err != nil) != (field != "") {
				t.Fatalf("BuildQuick error = %v", err)
			}
			if err != nil {
				return
			}
			if txn.ID != "id" || txn.AccountID != tc.account || txn.Amount.Amount != tc.minor || txn.Amount.Currency != strings.ToUpper(s.Accounts[map[string]int{"usd": 0, "jpy": 1}[tc.account]].Currency) {
				t.Fatalf("transaction = %+v", txn)
			}
			if txn.Desc != QuickDescription(tc.payee, tc.desc) || txn.Note != "note" || !txn.Reviewed || !txn.Cleared || !txn.ExcludeFromReports || txn.Source != domain.TxnSourceManual || !reflect.DeepEqual(txn.Tags, []string{"a", "b", "c"}) {
				t.Fatalf("metadata = %+v", txn)
			}
			if dateutil.FormatDate(txn.Date) != "2024-02-29" {
				t.Fatal(txn.Date)
			}
		})
	}
	for _, date := range []string{"", "2024-02-30", "bad", "2024-12-31"} {
		d := NewQuickDraft("active")
		d.Amount, d.Payee, d.Date, d.NeedsReview = "1", "P", date, true
		txn, err := BuildQuick(d, s.Accounts, "usd", "active", "id", s.Now)
		if err != nil || txn.Reviewed || !reflect.DeepEqual(txn.MemberIDs, []string{"active"}) {
			t.Fatalf("fallback %+v %v", txn, err)
		}
		if date != "2024-12-31" && !txn.Date.Equal(s.Now) {
			t.Fatal("fallback did not retain explicit local now")
		}
	}
}

func TestQuickDefaultsOwnersAndFocus(t *testing.T) {
	s := quickTestSource()
	for _, tc := range []struct {
		active, selected string
		explicit         bool
		want             string
	}{{"a", "b", false, "a"}, {"a", "b", true, "b"}, {"a", "", true, ""}} {
		if got := QuickMemberValue(tc.active, tc.selected, tc.explicit); got != tc.want {
			t.Fatal(got)
		}
	}
	for _, tc := range []struct {
		owners  []string
		active  string
		account domain.Account
		touched bool
		want    []string
	}{
		{nil, "", domain.Account{}, false, nil}, {nil, "a", domain.Account{}, false, []string{"a"}},
		{nil, "", s.Accounts[0], false, []string{"owner"}}, {nil, "", domain.Account{OwnerID: "owner"}, false, nil},
		{nil, "a", s.Accounts[0], true, nil}, {[]string{"b"}, "a", s.Accounts[0], false, []string{"b"}},
	} {
		got := QuickOwners(tc.owners, tc.active, tc.account, tc.touched)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("owners %v want %v", got, tc.want)
		}
		if len(got) > 0 {
			got[0] = "changed"
			if len(tc.owners) > 0 && tc.owners[0] == "changed" {
				t.Fatal("aliased owners")
			}
		}
	}
	for _, active := range []string{"", "a"} {
		d := NewQuickDraft(active)
		if d.Direction != "Expense" || !reflect.DeepEqual(d.Owners, QuickOwnerDefaults(active)) || !QuickReviewed(false) || QuickReviewed(true) {
			t.Fatal(d)
		}
	}
	for _, tc := range []struct {
		account, amount, payee, desc string
		fill                         smartcatview.DraftFill
		want                         string
	}{
		{"usd", "", "", "", smartcatview.DraftFill{}, "txn-add-amount"},
		{"bad", "1", "P", "", smartcatview.DraftFill{}, "txn-add-account"},
		{"usd", "1", " ", "", smartcatview.DraftFill{}, "txn-add-payee"},
		{"usd", "1", "P", "", smartcatview.DraftFill{}, "txn-add-save"},
		{"usd", "", "", "", smartcatview.DraftFill{AmountMajor: "1", Payee: "P"}, "txn-add-save"},
		{"usd", "1", "", "D", smartcatview.DraftFill{}, "txn-add-save"},
	} {
		if got := QuickNextFocus(s.Accounts, tc.account, tc.amount, tc.payee, tc.desc, tc.fill); got != tc.want {
			t.Fatalf("focus %s want %s", got, tc.want)
		}
	}
	if QuickHasAccount(nil, "usd") || QuickAmountValid(nil, "usd", "1", "Income") || QuickAccountID(accountselect.ResolveInput{}) != "" {
		t.Fatal("nil accounts")
	}
}

func TestQuickAccountResolution(t *testing.T) {
	accounts := []domain.Account{{ID: "a", Type: domain.TypeChecking, Class: domain.ClassAsset}, {ID: "b", Type: domain.TypeChecking, Class: domain.ClassAsset}, {ID: "old", Archived: true}}
	members := []domain.Member{{ID: "m", Prefs: domain.MemberPrefs{DefaultAccountID: "b"}}}
	for _, tc := range []struct {
		name  string
		input accountselect.ResolveInput
		want  string
	}{
		{"empty", accountselect.ResolveInput{}, ""},
		{"workspace", accountselect.ResolveInput{Accounts: accounts, WorkspaceDefaultID: "a"}, "a"},
		{"member", accountselect.ResolveInput{Accounts: accounts, Members: members, ActiveMemberID: "m", WorkspaceDefaultID: "a"}, "b"},
		{"explicit beats defaults", accountselect.ResolveInput{Accounts: accounts, Members: members, ActiveMemberID: "m", WorkspaceDefaultID: "b", ExplicitID: "a"}, "a"},
		{"archived default skipped", accountselect.ResolveInput{Accounts: accounts, WorkspaceDefaultID: "old"}, "a"},
		{"missing default skipped", accountselect.ResolveInput{Accounts: accounts, WorkspaceDefaultID: "missing"}, "a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := QuickAccountID(tc.input); got != tc.want {
				t.Fatalf("account %q want %q", got, tc.want)
			}
		})
	}
}

func TestQuickTransitions(t *testing.T) {
	s := quickTestSource()
	for _, tc := range []struct {
		id, direction string
		valid         bool
	}{{"out", "Expense", true}, {"in", "Income", true}, {"in", "Expense", false}, {"missing", "Income", false}} {
		if QuickCategoryValid(s.Categories, tc.id, tc.direction) != tc.valid {
			t.Fatal(tc)
		}
		d := NewQuickDraft("")
		d.CategoryID = tc.id
		next := QuickSetDirection(d, tc.direction, s.Categories)
		if (next.CategoryID != "") != tc.valid || d.CategoryID != tc.id {
			t.Fatalf("direction %+v", next)
		}
	}
	for _, raw := range []string{"1", "-1", " - 1", "", "-0"} {
		d := NewQuickDraft("")
		d.Direction, d.CategoryID = "Income", "in"
		next := QuickInputAmount(d, raw, s.Categories)
		if next.Amount != raw || (next.Direction == "Expense") != strings.HasPrefix(strings.TrimSpace(raw), "-") {
			t.Fatal(next)
		}
	}
	for _, tc := range []struct{ raw, want string }{{"2*3", "6.00"}, {"(12+8)*2", "40.00"}, {" -12.50 ", "12.50"}, {"-2*3", "2*3"}, {"oops", "old"}, {"12.5", "old"}, {"1/0", "old"}} {
		d := NewQuickDraft("")
		d.Amount = "old"
		if got := QuickBlurAmount(d, tc.raw).Amount; got != tc.want {
			t.Fatalf("blur %q=%q want %q", tc.raw, got, tc.want)
		}
	}
	for _, minor := range []int64{0, 1, -125, math.MaxInt64, math.MinInt64} {
		d := NewQuickDraft("")
		d.Amount, d.Desc, d.AccountID = "old", "old desc", "usd"
		template := domain.TxnTemplate{AmountMinor: minor, Currency: "USD", Payee: "P", CategoryID: "out", Direction: domain.DirectionExpense}
		got := QuickApplyTemplate(d, template, s.Categories)
		if got.AccountID != "usd" || got.Desc != "old desc" || got.Payee != "P" || got.CategoryID != "out" {
			t.Fatal(got)
		}
		if minor == 0 && got.Amount != "old" {
			t.Fatal("zero overwrote amount")
		}
		if minor == math.MinInt64 && got.Amount != "-92233720368547758.08" {
			t.Fatal("overflow magnitude changed money")
		}
	}
	d := NewQuickDraft("")
	d.CategoryID = "out"
	got := QuickApplyTemplate(d, domain.TxnTemplate{AccountID: "jpy", AmountMinor: 2, Currency: "JPY", Note: "note", Direction: domain.DirectionIncome, CategoryID: "out"}, s.Categories)
	if got.Direction != "Income" || got.CategoryID != "" || got.Amount != "2" || got.Desc != "note" || got.AccountID != "jpy" {
		t.Fatal(got)
	}
	for _, fill := range []smartcatview.DraftFill{{}, {Payee: "New"}, {AmountMajor: "2", CategoryID: "out"}, {Income: true, CategoryID: "in", Date: s.Now}} {
		d := NewQuickDraft("")
		d.Desc = "existing"
		d.Amount = "3"
		got := QuickApplyFill(d, fill, s.Categories)
		if got.Desc != "existing" || (got.Direction == "Income") != fill.Income {
			t.Fatal(got)
		}
		if fill.AmountMajor == "" && got.Amount != "3" {
			t.Fatal("empty fill erased amount")
		}
		if !fill.Date.IsZero() && got.Date != "2024-02-29" {
			t.Fatal(got.Date)
		}
	}
	d = NewQuickDraft("")
	if got := QuickApplyFill(d, smartcatview.DraftFill{Payee: "New", CategoryID: "missing"}, s.Categories); got.Desc != "New" || got.CategoryID != "" {
		t.Fatal(got)
	}
}

func TestQuickPreview(t *testing.T) {
	s := quickTestSource()
	for _, tc := range []struct {
		account, raw, direction string
		minor                   int64
		note                    string
		amountError             bool
	}{
		{"usd", "-12.50", "Income", -1250, "12.50", false}, {"jpy", "12", "Expense", 12, "", false},
		{"usd", "0", "Expense", 0, "", true}, {"usd", "bad", "Expense", 0, "", true}, {"usd", "92233720368547758.08", "Income", 0, "", true},
		{"usd", "-92233720368547758.08", "Expense", math.MinInt64, "-92233720368547758.08", false},
	} {
		d := NewQuickDraft("")
		d.AccountID, d.Amount, d.Direction = tc.account, tc.raw, tc.direction
		v := QuickPreview(d, s, true)
		if v.AccountID != tc.account || v.ImpactMinor != tc.minor || v.TemplateMinor != tc.minor || v.AmountNote != tc.note || v.AmountError != tc.amountError || !v.PayeeError || v.Date != "2024-02-29" {
			t.Fatalf("preview %+v", v)
		}
		if (v.TemplateDirection == domain.DirectionIncome) != (tc.direction == "Income") {
			t.Fatal(v.TemplateDirection)
		}
	}
	v := QuickPreview(NewQuickDraft(""), QuickSource{Now: s.Now}, true)
	if !v.AccountError || !v.AmountError || v.ImpactMinor != 0 || v.Currency != "" {
		t.Fatal(v)
	}
	d := NewQuickDraft("")
	d.Date = "2024-12-31"
	d.Desc = "plain"
	d.Payee = "Market"
	s.MemberLens = []string{"owner"}
	s.LearnTally = learntally.Tally{"market": {"out": 3}}
	v = QuickPreview(d, s, false)
	if v.LearnedCategoryID != "out" || v.LearnedCategoryName != "Food" || v.Date != d.Date || v.AccountError || v.AmountError || v.PayeeError || len(v.Members) != 1 || v.Colors["owner"] != "red" {
		t.Fatal(v)
	}
	d.Payee = ""
	d.Desc = "Market"
	if v := QuickPreview(d, s, false); v.LearnedCategoryID != "out" {
		t.Fatal("description fallback")
	}
	s.Rules = []rules.Rule{{Match: "Market", SetCategoryID: "out"}}
	if v := QuickPreview(d, s, false); v.CategorySuggestion != "Food" || v.LearnedCategoryName != "" {
		t.Fatal(v)
	}
	d.CategoryID = "out"
	if v := QuickPreview(d, s, false); v.CategorySuggestion != "" || v.LearnedCategoryName != "" {
		t.Fatal("selected category still suggested")
	}
	d.CategoryID = ""
	d.Direction = "Income"
	if v := QuickPreview(d, s, false); v.CategorySuggestion != "" || v.LearnedCategoryName != "" {
		t.Fatal("wrong-direction category suggested")
	}
	s.Rules = nil
	if v := QuickPreview(d, s, false); v.LearnedCategoryName != "" {
		t.Fatal("wrong-direction learned category")
	}
	d.Desc = "POS STARBUCKS #1234"
	d.Direction = "Expense"
	if v := QuickPreview(d, s, false); v.DescriptionSuggestion == "" {
		t.Fatal("missing merchant clean-up")
	}
}

func TestQuickPreviewMessagesAndVisibility(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		source                          QuickSource
		attempted                       bool
		accountKey, amountKey, payeeKey string
		members, templates              bool
	}{
		{name: "empty untouched"},
		{name: "empty attempted", attempted: true, accountKey: "txu3.needAccount", amountKey: "txu3.needAmount", payeeKey: "r4f.addPayeeOrDescription"},
		{name: "saved templates", source: QuickSource{TemplateCount: 1}, templates: true},
		{name: "past transactions", source: QuickSource{Transactions: []domain.Transaction{{Payee: "Market"}}}, templates: true},
		{name: "members", source: QuickSource{Members: []domain.Member{{ID: "m"}}}, members: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := QuickPreview(NewQuickDraft(""), tc.source, tc.attempted)
			if v.AccountErrorKey != tc.accountKey || v.AmountErrorKey != tc.amountKey || v.PayeeErrorKey != tc.payeeKey || v.ShowMembers != tc.members || v.ShowTemplates != tc.templates {
				t.Fatalf("preview %+v", v)
			}
		})
	}
	d := NewQuickDraft("")
	d.Payee, d.Desc = " Market ", " Bank text "
	v := QuickPreview(d, quickTestSource(), false)
	if v.TemplatePayee != "Market" || v.TemplateNote != "Bank text" {
		t.Fatal(v)
	}
}

func TestQuickDescription(t *testing.T) {
	for _, tc := range []struct{ payee, description, want string }{
		{" P ", " D ", "D"}, {" P ", " \t", "P"}, {"", " D ", "D"}, {" \n", "\t", ""},
	} {
		if got := QuickDescription(tc.payee, tc.description); got != tc.want {
			t.Fatalf("description %q want %q", got, tc.want)
		}
	}
}

func TestQuickChoiceTransitions(t *testing.T) {
	s := quickTestSource()
	for _, kind := range []string{"Expense", "Income", "Transfer"} {
		d := NewQuickDraft("owner")
		d.CategoryID = "out"
		next, transfer := QuickChooseDirection(d, kind, s.Categories)
		if transfer != (kind == "Transfer") {
			t.Fatal("wrong handoff")
		}
		if transfer && !reflect.DeepEqual(d, next) {
			t.Fatal("transfer changed the draft before handoff")
		}
		if !transfer && (next.Direction != kind || (next.CategoryID != "") != (kind == "Expense")) {
			t.Fatal(next)
		}
	}
	for _, owners := range [][]string{nil, {"owner"}, {"other"}} {
		d := NewQuickDraft("")
		d.Owners = owners
		next := QuickToggleOwner(d, "owner")
		if !next.OwnersTouched || !reflect.DeepEqual(next.Owners, ToggleOwner(owners, "owner")) {
			t.Fatal(next)
		}
		back := QuickToggleOwner(next, "owner")
		if !reflect.DeepEqual(back.Owners, owners) {
			t.Fatalf("double toggle %v want %v", back.Owners, owners)
		}
		if !reflect.DeepEqual(d.Owners, owners) || d.OwnersTouched {
			t.Fatal("input mutated")
		}
	}
}

func TestQuickPreviewHugeInputsAndStableOrder(t *testing.T) {
	s := quickTestSource()
	s.Categories = nil
	for i := 0; i < 10000; i++ {
		s.Categories = append(s.Categories, domain.Category{ID: strings.Repeat("x", i%3+1), Name: "Same", Kind: domain.KindExpense})
	}
	d := NewQuickDraft("")
	v := QuickPreview(d, s, false)
	for i, c := range v.Categories {
		if c.Value != s.Categories[i].ID {
			t.Fatal("equal labels reordered")
		}
	}
	// No FX table is required: every amount stays in the effective account's currency.
	for _, account := range s.Accounts {
		d.AccountID, d.Amount = account.ID, "12"
		v := QuickPreview(d, s, false)
		want, _ := money.ParseMinor("12", currency.Decimals(account.Currency))
		if v.Currency != account.Currency || v.ImpactMinor != want {
			t.Fatal("mixed currencies combined")
		}
	}
}

func FuzzQuickBuildRoundTrip(f *testing.F) {
	for _, n := range []int64{1, -1, 1250, math.MaxInt64, math.MinInt64} {
		f.Add(n, true)
	}
	f.Fuzz(func(t *testing.T, n int64, income bool) {
		d := NewQuickDraft("")
		d.Amount, d.Payee = money.FormatMinor(n, 2), "Payee"
		if income {
			d.Direction = "Income"
		}
		txn, err := BuildQuick(d, []domain.Account{{ID: "a", Currency: "USD"}}, "a", "", "id", time.Unix(0, 0))
		if n == 0 {
			if err == nil {
				t.Fatal("unrepresentable amount accepted")
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		want := n
		if !income && n > 0 {
			want = -n
		}
		if txn.Amount.Amount != want {
			t.Fatalf("got %d want %d", txn.Amount.Amount, want)
		}
		seed := Seed(txn, true)
		if !QuickAmountValid([]domain.Account{{ID: "a", Currency: "USD"}}, "a", seed.Amount, d.Direction) {
			t.Fatal("seed did not round-trip")
		}
	})
}

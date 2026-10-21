// SPDX-License-Identifier: MIT
package txnform

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/similartxns"
	"github.com/monstercameron/CashFlux/internal/txnmove"
)

func example() (domain.Transaction, []domain.Account) {
	return domain.Transaction{ID: "t", AccountID: "a", Payee: "SHOP", Desc: "bank", Amount: money.New(-100, "USD"), Date: time.Date(2024, 2, 29, 23, 59, 0, 0, time.FixedZone("west", -7*3600))}, []domain.Account{
		{ID: "a", Name: "Checking", Currency: "USD"}, {ID: "b", Name: "Savings", Currency: "USD"},
		{ID: "c", Name: "Euro", Currency: "EUR"}, {ID: "debt", Name: "Card", Currency: "USD", Class: domain.ClassLiability},
	}
}

func TestSeed(t *testing.T) {
	for _, tc := range []struct {
		name      string
		amount    int64
		found     bool
		want, dir string
	}{
		{"missing", -100, false, "", "out"}, {"expense", -100, true, "1.00", "out"}, {"income", 100, true, "1.00", "in"},
		{"zero", 0, true, "0.00", "out"}, {"minimum", math.MinInt64, true, "-92233720368547758.08", "out"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row, _ := example()
			row.Amount = money.New(tc.amount, "USD")
			row.Tags = []string{"a", "b"}
			row.BillAccountID = "debt"
			d := Seed(row, tc.found)
			if d.Amount != tc.want || d.Direction != tc.dir || d.Tags != "a, b" || !d.DebtPayment {
				t.Fatalf("seed = %+v", d)
			}
			if tc.found && d.Date != "2024-02-29" {
				t.Fatal(d.Date)
			}
			if !tc.found && d.Date != "" {
				t.Fatal(d.Date)
			}
		})
	}
}

func TestBuild(t *testing.T) {
	for _, tc := range []struct {
		name   string
		edit   func(*Draft, *domain.Transaction)
		key    string
		amount int64
		sync   bool
	}{
		{name: "unchanged", amount: -100},
		{name: "negative overrides income", edit: func(d *Draft, _ *domain.Transaction) { d.Direction = "in"; d.Amount = "-2" }, amount: -200, sync: true},
		{name: "income", edit: func(d *Draft, _ *domain.Transaction) { d.Direction = "in" }, amount: 100},
		{name: "zero", edit: func(d *Draft, _ *domain.Transaction) { d.Amount = "0" }, key: "transactions.positiveAmount"},
		{name: "junk", edit: func(d *Draft, _ *domain.Transaction) { d.Amount = "abc" }, key: "transactions.amountNotANumber"},
		{name: "overflow", edit: func(d *Draft, _ *domain.Transaction) { d.Amount = "92233720368547758.08" }, key: "transactions.amountNotANumber"},
		{name: "minimum magnitude", edit: func(d *Draft, _ *domain.Transaction) { d.Amount = "-92233720368547758.08" }, amount: math.MinInt64},
		{name: "huge amount input", edit: func(d *Draft, _ *domain.Transaction) { d.Amount = strings.Repeat("9", 100000) }, key: "transactions.amountNotANumber"},
		{name: "positive transfer retains historical minimum fallback", edit: func(d *Draft, row *domain.Transaction) {
			row.Amount = money.New(1, "USD")
			row.TransferAccountID = "b"
			d.TransferAccountID = "b"
			d.Amount = "-92233720368547758.08"
		}, amount: math.MinInt64},
		{name: "bad date", edit: func(d *Draft, _ *domain.Transaction) { d.Date = "2025-02-29" }, key: "transactions.invalidDate"},
		{name: "split mismatch", edit: func(_ *Draft, row *domain.Transaction) {
			row.Splits = []domain.CategorySplit{{Amount: money.New(-99, "USD")}}
		}, key: "transactions.splitAmountMismatch"},
		{name: "mixed split retains historical minor-unit reconciliation", edit: func(_ *Draft, row *domain.Transaction) {
			row.Splits = []domain.CategorySplit{{Amount: money.New(-100, "EUR")}}
		}, amount: -100},
		{name: "split overflow", edit: func(_ *Draft, row *domain.Transaction) {
			row.Splits = []domain.CategorySplit{{Amount: money.New(math.MaxInt64, "USD")}, {Amount: money.New(math.MaxInt64, "USD")}, {Amount: money.New(-98, "USD")}}
		}, key: "transactions.splitAmountMismatch"},
		{name: "currency move", edit: func(d *Draft, _ *domain.Transaction) { d.AccountID = "c" }, key: "transactions.accountMoveCurrency"},
		{name: "unknown move", edit: func(d *Draft, _ *domain.Transaction) { d.AccountID = "missing" }, key: "transactions.accountMoveRefused"},
		{name: "valid move", edit: func(d *Draft, _ *domain.Transaction) { d.AccountID = "b" }, amount: -100},
		{name: "transfer negative sign fixed", edit: func(d *Draft, row *domain.Transaction) {
			row.TransferAccountID = "b"
			d.TransferAccountID = "b"
			d.Direction = "in"
			d.Amount = "2"
		}, amount: -200},
		{name: "transfer positive sign fixed", edit: func(d *Draft, row *domain.Transaction) {
			row.Amount = money.New(100, "USD")
			row.TransferAccountID = "b"
			d.TransferAccountID = "b"
			d.Amount = "-2"
		}, amount: 200},
		{name: "self transfer move", edit: func(d *Draft, row *domain.Transaction) {
			row.TransferAccountID = "b"
			d.TransferAccountID = "b"
			d.AccountID = "b"
		}, key: "transactions.accountMoveSelf"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row, accounts := example()
			d := Seed(row, true)
			if tc.edit != nil {
				tc.edit(&d, &row)
			}
			before := row
			plan := Build(row, d, accounts, nil, time.Unix(9, 0))
			if plan.Failure.Key != tc.key {
				t.Fatalf("key=%q want %q; err=%v", plan.Failure.Key, tc.key, plan.Err)
			}
			if tc.key != "" {
				if plan.Err == nil {
					t.Fatal("missing error")
				}
				return
			}
			if plan.Err != nil || plan.Transaction.Amount.Amount != tc.amount || plan.SyncDirectionOut != tc.sync {
				t.Fatalf("plan=%+v", plan)
			}
			if !reflect.DeepEqual(row, before) {
				t.Fatal("mutated source")
			}
		})
	}
	t.Run("unclassifying transfer plans confirmation", func(t *testing.T) {
		row, accounts := example()
		row.TransferAccountID = "b"
		d := Seed(row, true)
		d.TransferAccountID = ""
		p := Build(row, d, accounts, nil, time.Now())
		if p.Err != nil || !p.UnclassifiesTransfer {
			t.Fatal(p)
		}
	})

	row, accounts := example()
	d := Seed(row, true)
	d.TransferAccountID = "missing"
	if p := Build(row, d, accounts, nil, time.Now()); p.Err == nil || p.Failure.Key != "" {
		t.Fatalf("classification: %+v", p)
	}
	d = Seed(row, true)
	d.Desc = " new "
	d.Payee = " clean "
	d.Note = " memo "
	d.Tags = " review, foo "
	d.CategoryID = "food"
	d.Cleared = true
	d.Exclude = true
	d.Owners = []string{"m", "m"}
	d.DebtPayment = true
	d.TransferAccountID = "b"
	p := Build(row, d, accounts, nil, time.Unix(9, 0))
	got := p.Transaction
	if p.Err != nil || got.Desc != "new" || got.Payee != "clean" || got.Note != "memo" || !got.Reviewed || !got.Cleared || !got.ExcludeFromReports || len(got.Owners()) != 1 || got.BillAccountID != "" {
		t.Fatalf("draft application: %+v %v", got, p.Err)
	}
}

func TestSuggestionTransaction(t *testing.T) {
	for _, tc := range []struct {
		raw, dir, cur string
		want          int64
	}{
		{"-1", "in", "USD", 100}, {"1", "out", "USD", -100}, {"abc", "in", "USD", 0}, {"0", "in", "USD", 0},
		{"92233720368547758.08", "out", "USD", 0}, {"-92233720368547758.08", "in", "USD", math.MinInt64}, {"10", "in", "JPY", 10},
	} {
		row, _ := example()
		row.Amount.Currency = tc.cur
		got := SuggestionTransaction(row, tc.raw, tc.dir)
		if got.Amount.Amount != tc.want || got.ID != row.ID {
			t.Fatalf("%+v: %+v", tc, got)
		}
	}
}

func TestCategories(t *testing.T) {
	cats := []domain.Category{{ID: "ten", Name: "Item 10", Kind: domain.KindExpense}, {ID: "nine", Name: "Item 9", Kind: domain.KindExpense}, {ID: "child", Name: "Child", ParentID: "nine", Kind: domain.KindExpense}, {ID: "income", Kind: domain.KindIncome}, {ID: "equal", Name: "Item 9", Kind: domain.KindExpense}}
	for _, tc := range []struct {
		dir  string
		kind domain.CategoryKind
		ids  []string
	}{{"out", domain.KindExpense, []string{"nine", "equal", "child", "ten"}}, {"in", domain.KindIncome, []string{"income"}}} {
		opts, kind := Categories(cats, tc.dir)
		var ids []string
		for _, o := range opts {
			ids = append(ids, o.Value)
		}
		if kind != tc.kind || !reflect.DeepEqual(ids, tc.ids) {
			t.Fatalf("options=%+v kind=%v", opts, kind)
		}
	}
	if opts, _ := Categories(nil, ""); len(opts) != 0 {
		t.Fatal(opts)
	}
	huge := make([]domain.Category, 2048)
	for i := range huge {
		huge[i] = domain.Category{ID: "a", Name: "Same", Kind: domain.KindExpense}
	}
	opts, _ := Categories(huge, "out")
	if len(opts) != len(huge) {
		t.Fatal(len(opts))
	}
}

func TestSplits(t *testing.T) {
	for _, tc := range []struct {
		cats       []domain.Category
		members    []domain.Member
		cat, owner string
	}{
		{nil, nil, "", "missing"}, {[]domain.Category{{ID: "c", Name: "Food"}}, []domain.Member{{ID: "missing", Name: "Cam"}}, "Food", "Cam"},
	} {
		row := domain.Transaction{Splits: []domain.CategorySplit{{CategoryID: "c", MemberID: "missing", Amount: money.New(-100, "USD")}, {Amount: money.New(20, "EUR")}}}
		lines := Splits(row, tc.cats, tc.members)
		if len(lines) != 2 || lines[0].Category != tc.cat || lines[0].Owner != tc.owner || lines[1].Owner != "" || lines[1].Amount.Currency != "EUR" {
			t.Fatal(lines)
		}
	}
	if len(Splits(domain.Transaction{}, nil, nil)) != 0 {
		t.Fatal("empty")
	}
}

func TestSimilarWrites(t *testing.T) {
	row, _ := example()
	row.Tags = []string{"review", "keep"}
	transfer := row
	transfer.ID = "transfer"
	transfer.TransferAccountID = "b"
	cands := []similartxns.Candidate{{Txn: domain.Transaction{ID: "missing"}}, {Txn: transfer}, {Txn: row}}
	writes, ids := SimilarWrites(cands, []domain.Transaction{row, transfer}, "food")
	if len(writes) != 1 || !reflect.DeepEqual(ids, []string{"t"}) || !writes[0].Reviewed || writes[0].CategoryID != "food" {
		t.Fatalf("%+v %v", writes, ids)
	}
	writes[0].Tags[0] = "changed"
	if row.Tags[0] != "review" {
		t.Fatal("mutated live tags")
	}
	if w, i := SimilarWrites(nil, nil, ""); len(w) != 0 || len(i) != 0 {
		t.Fatal(w, i)
	}
}

func TestDecisions(t *testing.T) {
	_, accounts := example()
	for _, tc := range []struct {
		id   string
		want bool
	}{{"", false}, {"missing", false}, {"a", false}, {"debt", true}} {
		if IsLiability(accounts, tc.id) != tc.want {
			t.Fatal(tc)
		}
	}
	for _, tc := range []struct {
		reason txnmove.Reason
		key    string
	}{{txnmove.ReasonCurrency, "transactions.accountMoveCurrency"}, {txnmove.ReasonSelfTransfer, "transactions.accountMoveSelf"}, {txnmove.ReasonUnknownAccount, "transactions.accountMoveRefused"}} {
		if MoveErrorKey(tc.reason) != tc.key {
			t.Fatal(tc)
		}
	}
	for _, tc := range []struct {
		row  domain.Transaction
		want string
	}{{domain.Transaction{Payee: " p ", Desc: "d"}, "p"}, {domain.Transaction{Desc: " d "}, "d"}, {domain.Transaction{Payee: " ", Desc: "d"}, ""}} {
		if Phrase(tc.row) != tc.want {
			t.Fatal(tc)
		}
	}
	for _, tc := range []struct {
		old, newName  string
		learned, want bool
	}{{" a ", " b ", false, true}, {"a", "A", false, false}, {"", "b", false, false}, {"a", " ", false, false}, {"a", "b", true, false}} {
		old, newName, offer := LearnAlias(tc.old, tc.newName, tc.learned)
		if old != strings.TrimSpace(tc.old) || newName != strings.TrimSpace(tc.newName) || offer != tc.want {
			t.Fatal(tc)
		}
	}
	for _, tc := range []struct {
		open, has bool
		key       string
	}{{true, true, "splitEditor.hide"}, {false, true, "splitEditor.edit"}, {false, false, "splitEditor.toggle"}} {
		if SplitKey(tc.open, tc.has) != tc.key {
			t.Fatal(tc)
		}
	}
	for _, tc := range []struct {
		n   int
		key string
	}{{0, "transactions.receiptsAttached"}, {1, "transactions.receiptAttached"}, {2, "transactions.receiptsAttached"}} {
		if ReceiptKey(tc.n) != tc.key {
			t.Fatal(tc)
		}
	}
	for _, tc := range []struct{ msg, amount, date string }{{"positive", "positive", ""}, {"number", "number", ""}, {"date", "", "date"}, {"other", "", ""}, {"", "", ""}} {
		a, d := FieldErrors(tc.msg, "positive", "number", "date")
		if a != tc.amount || d != tc.date {
			t.Fatal(tc, a, d)
		}
	}
	for _, tc := range []struct {
		owners []string
		owner  string
		want   []string
	}{{nil, "a", []string{"a"}}, {[]string{"a", "b"}, "a", []string{"b"}}, {[]string{"a"}, "b", []string{"a", "b"}}} {
		got := ToggleOwner(tc.owners, tc.owner)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatal(got)
		}
		got[0] = "changed"
		if len(tc.owners) > 0 && tc.owners[0] == "changed" {
			t.Fatal("mutated owners")
		}
	}
}

func TestThumbnails(t *testing.T) {
	refs := []domain.AttachmentRef{{ArtifactID: "ok"}, {ArtifactID: "missing"}, {ArtifactID: "empty"}}
	got := Thumbnails(refs, []domain.Artifact{{ID: "ok", MIME: "image/png", Bytes: []byte("a")}, {ID: "empty"}})
	if len(got) != 3 {
		t.Fatal(got)
	}
	if got[0].Artifact.ID != "ok" || string(got[0].Artifact.Bytes) != "a" || got[1].Artifact.ID != "" || got[2].Artifact.ID != "empty" {
		t.Fatal("thumbnail artifact metadata was not retained")
	}
	if len(Thumbnails(nil, nil)) != 0 {
		t.Fatal("empty")
	}
}

func TestSparklinePoints(t *testing.T) {
	for _, tc := range []struct {
		series []int64
		want   string
	}{
		{nil, ""}, {[]int64{1}, ""}, {[]int64{1, 1}, "0.0,12.0 120.0,12.0"},
		{[]int64{-100, 0, 100}, "0.0,23.0 60.0,12.0 120.0,1.0"},
		{[]int64{100, 0, -100}, "0.0,1.0 60.0,12.0 120.0,23.0"},
		{[]int64{math.MinInt64, math.MaxInt64}, "0.0,12.0 120.0,12.0"},
	} {
		if got := SparklinePoints(tc.series); got != tc.want {
			t.Fatalf("%v: %q", tc.series, got)
		}
	}
}

func TestAccountChoices(t *testing.T) {
	row, accounts := example()
	sorted, choices := AccountChoices(accounts, row)
	if len(sorted) != 4 || len(choices) != 3 {
		t.Fatal(sorted, choices)
	}
	if a, c := AccountChoices(nil, row); len(a) != 0 || len(c) != 0 {
		t.Fatal(a, c)
	}
}

func FuzzDraftRoundTrip(f *testing.F) {
	for _, v := range []int64{-100, 1, 0, math.MaxInt64, math.MinInt64} {
		f.Add(v)
	}
	f.Fuzz(func(t *testing.T, n int64) {
		row, accounts := example()
		row.Amount = money.New(n, "USD")
		d := Seed(row, true)
		p := Build(row, d, accounts, nil, time.Unix(0, 0))
		if n == 0 {
			if p.Err == nil {
				t.Fatal("invalid magnitude accepted")
			}
			return
		}
		if p.Err != nil || p.Transaction.Amount != row.Amount || p.Transaction.Date.Format("2006-01-02") != row.Date.Format("2006-01-02") {
			t.Fatalf("round trip %d: %+v", n, p)
		}
	})
}

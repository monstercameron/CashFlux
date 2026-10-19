package txntable

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txnview"
)

func TestSpecsAndPreview(t *testing.T) {
	n := NativeSpec("txn-toolbar")
	if n.ID != "txn-toolbar" || n.NativeID != n.ID || n.Kind != domain.KindNative || n.SchemaVersion != domain.WidgetSpecVersion {
		t.Fatal(n)
	}
	table := TableSpec()
	if table.ID != "txn-table" || table.Kind != domain.KindTable || table.Pipeline.Source.Collection != "transactions-full" {
		t.Fatal(table)
	}
	specs := SurfaceSpecs([]string{"txn-toolbar", "txn-table"})
	if len(specs) != 2 || !reflect.DeepEqual(specs[0], n) || !reflect.DeepEqual(specs[1], table) {
		t.Fatal(specs)
	}
	if len(SurfaceSpecs(nil)) != 0 {
		t.Fatal("nil specs")
	}
	arts := []domain.Artifact{{ID: "a", Name: "Receipt"}}
	got := PreviewArtifact("a", arts)
	if got == nil || got.ID != "a" {
		t.Fatal(got)
	}
	got.Name = "changed"
	if arts[0].Name != "Receipt" {
		t.Fatal("aliased input")
	}
	if PreviewArtifact("missing", arts) != nil || PreviewArtifact("a", nil) != nil {
		t.Fatal("missing artifact")
	}
}

func TestDeletePromptAndOwnerLabels(t *testing.T) {
	rows := []domain.Transaction{{ID: "a", AccountID: "bank", Payee: "Merchant", Date: time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC), Amount: money.New(-100, "USD")}, {ID: "b", Desc: "Transfer", Amount: money.New(100, "USD")}}
	accounts := []domain.Account{{ID: "bank", Name: "Checking"}}
	want := "transactions.deleteConfirmDetail:Merchant2026-01-31-100 USDChecking transactions.deleteConfirmDetail:Transfer0001-01-01100 USD"
	if got := DeletePrompt([]string{"a", "missing", "b"}, rows, accounts, 0, labels()); got != want {
		t.Fatalf("%q", got)
	}
	if got := DeletePrompt([]string{"a", "b"}, rows, accounts, 2, labels()); got != want+" transactions.deleteConfirmTasks:2" {
		t.Fatal(got)
	}
	if DeletePrompt(nil, nil, nil, 0, labels()) != "" {
		t.Fatal("empty")
	}
	for _, tt := range []struct {
		names []string
		want  string
	}{{nil, "who2.unassigned"}, {[]string{"Cam"}, "Cam"}, {[]string{"Cam", "Amy"}, "who2.sharedBy:Cam and Amy"}, {[]string{"A", "B", "C"}, "who2.sharedBy:A, B, and C"}} {
		if got := OwnerLabel(tt.names, translate); got != tt.want {
			t.Fatal(got)
		}
	}
	if label, assigned := LegacyOwnerLabel(" ", translate); label != "r4c.unassigned" || assigned {
		t.Fatal(label, assigned)
	}
	if label, assigned := LegacyOwnerLabel("Cam", translate); label != "txu5.member:Cam" || !assigned {
		t.Fatal(label, assigned)
	}
}

func TestSuggestionAndTagNotice(t *testing.T) {
	categories := []domain.Category{{ID: "expense", Kind: domain.KindExpense}, {ID: "income", Kind: domain.KindIncome}}
	names := map[string]string{"expense": "Food", "income": "Salary"}
	row := domain.Transaction{Amount: money.New(-100, "USD")}
	for _, tt := range []struct {
		id         string
		confidence catsuggest.Confidence
		found      bool
		want       string
	}{{"expense", catsuggest.ConfMedium, true, "Food"}, {"expense", catsuggest.ConfHigh, true, "Food"}, {"expense", catsuggest.ConfLow, true, ""}, {"income", catsuggest.ConfHigh, true, ""}, {"missing", catsuggest.ConfHigh, true, ""}, {"expense", catsuggest.ConfHigh, false, ""}} {
		if got := SuggestedCategory(row, catsuggest.Suggestion{CategoryID: tt.id, Confidence: tt.confidence}, tt.found, categories, names); got != tt.want {
			t.Fatal(tt, got)
		}
	}
	if SuggestedCategory(row, catsuggest.Suggestion{}, true, nil, nil) != "" {
		t.Fatal("nil")
	}
	rows := []domain.Transaction{{ID: "a", Tags: []string{"tag"}}, {ID: "b"}}
	next := txnfilter.Criteria{Tag: "tag"}
	for _, tt := range []struct {
		previous, next txnfilter.Criteria
		shown          int
		want           bool
	}{{txnfilter.Criteria{}, next, 1, true}, {txnfilter.Criteria{}, next, 2, false}, {next, next, 2, true}} {
		if got := TagFilterNotice(tt.previous, tt.next, next, rows, txnfilter.Labels{}, tt.shown); got != tt.want {
			t.Fatal(tt)
		}
	}
}

func TestCountFacts(t *testing.T) {
	count := func(n int) string { return fmt.Sprint(n) }
	for _, tt := range []struct {
		cached []txnview.View
		want   int
	}{{nil, 1}, {[]txnview.View{{Transferred: 100, UnpairedRows: 2, Unconverted: 3, Excluded: 4}}, 5}, {[]txnview.View{{}, {}}, 1}} {
		got := CountFacts(nil, "USD", nil, tt.cached, labels(), count)
		if len(got) != tt.want || got[0].Text != "0" || got[0].TestID != "txn-count-transaction-total" {
			t.Fatal(got)
		}
		if tt.want == 5 {
			if got[1].Text != "transactions.scopeMoved:100 USD" || got[2].Class != "txn-count-fact txn-count-unpaired" || got[2].TestID != "" || got[3].TestID != "txn-count-unconverted" || got[4].TestID != "txn-count-excluded" {
				t.Fatal(got)
			}
		}
	}
	rows := []domain.Transaction{{ID: "fx", Amount: money.New(-100, "EUR")}, {ID: "excluded", Amount: money.New(-50, "USD"), ExcludeFromReports: true}}
	got := CountFacts(rows, "USD", nil, nil, labels(), count)
	if got[0].Text != "2" || len(got) != 3 {
		t.Fatal(got)
	}
}

func TestDeriveRowsMissingFXAndLargeInput(t *testing.T) {
	rows := make([]domain.Transaction, 100000)
	for i := range rows {
		rows[i] = domain.Transaction{ID: fmt.Sprintf("%06d", len(rows)-i), Desc: "same", Payee: "raw", Amount: money.New(-10, "USD")}
	}
	calls := 0
	result, issues := DeriveRows(rows, txnfilter.Criteria{Text: "clean", Sort: "payee", Dir: txnfilter.Asc}, txnfilter.Labels{Payee: func(s string) string { calls++; return "clean" }}, nil, nil)
	if len(result) != len(rows) || len(issues) != 0 || result[0].ID != "000001" || calls != 1 {
		t.Fatal(len(result), len(issues), calls)
	}
	rates := currency.Rates{Base: "USD"}
	_, issues = DeriveRows([]domain.Transaction{{ID: "missing", Amount: money.New(-100, "EUR")}}, txnfilter.Criteria{AmountMin: "0.01"}, txnfilter.Labels{BaseCurrency: "USD", ToBase: func(m money.Money) (money.Money, error) { return rates.Convert(m, "USD") }}, nil, nil)
	if len(issues) != 1 {
		t.Fatalf("missing FX issues=%d", len(issues))
	}
}

func FuzzDescriptionSortIsDeterministic(f *testing.F) {
	f.Add("Coffee,coffee,Market", true)
	f.Fuzz(func(t *testing.T, input string, descending bool) {
		if len(input) > 2048 {
			return
		}
		parts := strings.Split(input, ",")
		rows := make([]domain.Transaction, len(parts))
		for i, desc := range parts {
			rows[i] = domain.Transaction{ID: fmt.Sprintf("%06d", i), Desc: desc}
		}
		dir := txnfilter.Asc
		if descending {
			dir = txnfilter.Desc
		}
		criteria := txnfilter.Criteria{Sort: "payee", Dir: dir}
		got, _ := DeriveRows(rows, criteria, txnfilter.Labels{}, nil, nil)
		again, _ := DeriveRows(got, criteria, txnfilter.Labels{}, nil, nil)
		if !reflect.DeepEqual(got, again) {
			t.Fatal("sort not idempotent")
		}
		for i := 1; i < len(got); i++ {
			left, right := strings.ToLower(Description(got[i-1].Desc, got[i-1].Payee, nil, nil)), strings.ToLower(Description(got[i].Desc, got[i].Payee, nil, nil))
			if (left == right && got[i-1].ID > got[i].ID) || (left != right && ((descending && left < right) || (!descending && left > right))) {
				t.Fatal("sort order")
			}
		}
	})
}

package txntable

import (
	"fmt"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func translate(key string, args ...any) string {
	if len(args) == 0 {
		return key
	}
	return key + ":" + fmt.Sprint(args...)
}
func labels() Labels {
	return Labels{T: translate, Money: func(m money.Money) string { return fmt.Sprint(m.Amount, " ", m.Currency) }, Ledger: func(m money.Money, transfer bool) (string, string) { return fmt.Sprint(m.Amount), fmt.Sprint(transfer) }}
}

func TestDisplayDecisions(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for _, tt := range []struct{ from, to, want string }{{"2026-01-01", "2026-12-31", "Feb 28"}, {"2025-01-01", "2025-12-31", "Feb 28, 2026"}, {"2026-01-01", "2027-01-01", "Feb 28, 2026"}, {"bad", "2026-12-31", "Feb 28, 2026"}, {"2026-01-01", "bad", "Feb 28, 2026"}} {
		if got := LedgerDate(time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC), tt.from, tt.to, now); got != tt.want {
			t.Fatal(got)
		}
	}
	west := time.FixedZone("west", -5*3600)
	if got := LedgerDate(time.Date(2026, 1, 31, 23, 0, 0, 0, west), "", "", now); got != "Jan 31, 2026" {
		t.Fatal(got)
	}
	for _, amount := range []int64{math.MinInt64, -1, 0, 1, math.MaxInt64} {
		kind := domain.KindExpense
		key := "r4c.transferFrom"
		if amount > 0 {
			kind = domain.KindIncome
		}
		if amount < 0 {
			key = "r4c.transferTo"
		}
		if CategoryKind(money.New(amount, "USD")) != kind || TransferCategoryKey(money.New(amount, "USD")) != key {
			t.Fatal(amount)
		}
	}
	if NeedsTooltip(strings.Repeat("\u00e9", 23), 23) || !NeedsTooltip(strings.Repeat("\u00e9", 24), 23) {
		t.Fatal("runes")
	}
	if got := OwnerNames([]string{"a", "b", "c"}, map[string]string{"a": " Cam ", "b": " "}); !slices.Equal(got, []string{"Cam", "b", "c"}) {
		t.Fatal(got)
	}
	if got := OwnerNames([]string{"a"}, nil); !slices.Equal(got, []string{"a"}) {
		t.Fatal(got)
	}
	if got := OwnerColors([]string{"a", "b"}, map[string]string{"a": "blue"}); !slices.Equal(got, []string{"blue", ""}) {
		t.Fatal(got)
	}
	if len(OwnerColors(nil, nil)) != 0 || OwnerColor(nil, 0) != "" || OwnerColor([]string{"red"}, -1) != "" || OwnerColor([]string{"red"}, 0) != "red" {
		t.Fatal("colors")
	}
	for mask := 0; mask < 8; mask++ {
		selected, cleared, excluded := mask&1 != 0, mask&2 != 0, mask&4 != 0
		want := "row txn-ledger-row"
		if selected {
			want += " selected"
		}
		if cleared {
			want += " cleared"
		}
		if excluded {
			want += " txn-excluded"
		}
		if RowClass(selected, cleared, excluded) != want {
			t.Fatal(mask)
		}
	}
	for _, s := range []string{"", "\u2014", "Imported"} {
		want := "td-source"
		if s != "Imported" {
			want += " text-dim"
		}
		if SourceClass(s) != want {
			t.Fatal(s)
		}
	}
	for n := 0; n < 6; n++ {
		tags := make([]string, n)
		got, extra := TagWindow(tags)
		if len(got) != min(n, 3) || extra != max(0, n-3) {
			t.Fatal(n)
		}
	}
	if name, title := CategoryDisplay("short", "", "", false, false, translate); name != "short" || title != "" {
		t.Fatal(name, title)
	}
	if name, title := CategoryDisplay("short", "Food", "", true, false, translate); name != "r5w.suggestedCategory:shortFood" || title != name {
		t.Fatal(name, title)
	}
	if _, title := CategoryDisplay(strings.Repeat("x", 24), "Food", "", true, false, translate); title != strings.Repeat("x", 24) {
		t.Fatal(title)
	}
	if _, title := CategoryDisplay(strings.Repeat("x", 24), "", "tip", false, true, translate); title != "tip" {
		t.Fatal(title)
	}
	if _, title := CategoryDisplay(strings.Repeat("x", 24), "", "", false, true, translate); title != "" {
		t.Fatal(title)
	}
	if ActionName("edit", "coffee", "date", "money", translate) != "txw1.rowAction:editcoffeedatemoney" {
		t.Fatal("action")
	}
	for _, tt := range []struct {
		d, r, s bool
		want    Badge
	}{{true, true, true, BadgeDuplicate}, {false, true, true, BadgeReview}, {false, false, true, BadgeSpike}, {false, false, false, BadgeNone}} {
		if DescriptionBadge(tt.d, tt.r, tt.s) != tt.want {
			t.Fatal(tt)
		}
	}
	if Info("a", " ") != "a" || Info("", "b") != "b" || Info("a", "b") != "a. b" {
		t.Fatal("info")
	}
	if BaseCurrency("") != "USD" || BaseCurrency("EUR") != "EUR" {
		t.Fatal("base")
	}
	for _, sel := range []bool{false, true} {
		for _, cal := range []bool{false, true} {
			ids := SurfaceIDs(sel, cal)
			want := []string{"txn-toolbar"}
			class := "bento bento-ledger"
			if sel {
				want = append(want, "txn-bulkbar")
				class += " txn-has-selection"
			}
			if cal {
				want = append(want, "txn-calendar")
			} else {
				want = append(want, "txn-table")
			}
			if !slices.Equal(ids, want) || SurfaceClass(sel) != class {
				t.Fatal(ids)
			}
		}
	}
	if ExcludeNoticeKey(false) != "transactions.includedOne" || ExcludeNoticeKey(true) != "transactions.excludedOne" {
		t.Fatal("exclude")
	}
	for _, tt := range []struct {
		from, to string
		out      bool
		want     string
	}{{"A", "B", true, "A \u2192 B"}, {"A", "B", false, "B \u2192 A"}, {" ", "B", true, ""}, {"A", "", true, ""}} {
		if TransferLabel(tt.from, tt.to, tt.out) != tt.want {
			t.Fatal(tt)
		}
	}
}

func TestRowDerivation(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	tx := domain.Transaction{ID: "a", AccountID: "bank", Date: now, Amount: money.New(-125, "USD"), CategoryID: "food", MemberIDs: []string{"m"}, Reviewed: true, Note: "note", Attachments: []domain.AttachmentRef{{ArtifactID: "receipt"}}, Tags: []string{"tag"}}
	x := BuildIndex([]domain.Transaction{tx}, []domain.Account{{ID: "bank", Name: "Bank", Currency: "USD", Reconciliations: []domain.Reconciliation{{At: now}}}}, []domain.Category{{ID: "food", Name: "Food"}}, []domain.Member{{ID: "m", Name: "Cam", Color: "blue"}}, nil, nil)
	ctx := RowContext{Index: x, Now: now, Labels: labels(), Filter: txnfilter.Criteria{From: "2026-10-01", To: "2026-10-31"}, Balances: map[string]money.Money{"a": money.New(50, "USD")}}
	in := RowInput{ID: "a", Description: "Coffee", Amount: tx.Amount, Date: now, Category: "Food", Cleared: true, Selected: true, FollowUpOpen: 1, FollowUpTotal: 2, SuggestionsReady: true}
	got := DeriveRow(ctx, in)
	if got.Desc != "Coffee" || got.Date != "Oct 1" || got.Amount != "-125" || got.Category != "Food" || !got.Selected || !got.Reconciled || !got.HasNote || got.Receipts != 1 || got.Attachment.ArtifactID != "receipt" || !got.ShowBalance || got.MemberNames[0] != "Cam" || !got.ShowSuggestions {
		t.Fatalf("%+v", got)
	}
	wantInfo := "uxt5.openFollowUp. uxt5.receipt. uxt5.reconciled"
	if got.InfoText != wantInfo {
		t.Fatalf("%q", got.InfoText)
	}
	if got.FlashKey != FlashKey(tx) {
		t.Fatal("flash")
	}
	before := FlashKey(tx)
	tx.Desc = "new"
	if before == FlashKey(tx) {
		t.Fatal("hash unchanged")
	}
	tx.Custom = map[string]any{"nan": math.NaN()}
	if FlashKey(tx) != "a" {
		t.Fatal("JSON fallback")
	}
	empty := DeriveRow(RowContext{Labels: labels(), Now: now}, RowInput{})
	if empty.Category != "transactions.uncategorized" || !empty.Unfiled || empty.Receipts != 0 || empty.ShowBalance {
		t.Fatalf("%+v", empty)
	}
}

func TestRowInfoCombinations(t *testing.T) {
	for mask := 0; mask < 1024; mask++ {
		r := Row{FollowUpTotal: 2, FollowUpOpen: 2, Receipts: 2, AutoCategory: mask&1 != 0, Queued: mask&2 != 0, Cleared: mask&4 != 0, Reconciled: mask&8 != 0, Duplicate: mask&16 != 0, SpikeWhy: "spike", SuspectTransfer: mask&32 != 0, EventName: "Trip", HasSplits: mask&64 != 0, IsDebtPayment: mask&128 != 0, GroupSize: 3, GroupTotal: "100", IsRefund: mask&256 != 0, IsRefunded: true, ExcludedFromReports: mask&512 != 0}
		info := rowInfo(translate, r, "split")
		if !strings.HasPrefix(info, "uxt5.openFollowUps:2. uxt5.receipts:2") {
			t.Fatal(info)
		}
		if strings.Contains(info, "uxt5.review") != (r.Queued && (r.Duplicate || r.SpikeWhy != "")) {
			t.Fatal(info)
		}
		if strings.Contains(info, "txnlinks.refundBadge") != r.IsRefund {
			t.Fatal(info)
		}
	}
	r := Row{IsDebtPayment: true, TransferWith: "Loan"}
	if !strings.Contains(rowInfo(translate, r, ""), "Loan") {
		t.Fatal("debt")
	}
	r = Row{}
	if rowInfo(translate, r, "") != "" {
		t.Fatal("empty")
	}
}

func TestRowTransfersSplitsGroupsAndMissingEnrichment(t *testing.T) {
	for _, transfer := range []bool{false, true} {
		tx := domain.Transaction{ID: "a", AccountID: "bank", Date: time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC), Amount: money.New(-100, "USD"), BillAccountID: "debt", Reviewed: true}
		if transfer {
			tx.TransferAccountID = "other"
		} else {
			tx.Splits = []domain.CategorySplit{{CategoryID: "c", Amount: money.New(-50, "USD")}, {CategoryID: "d", Amount: money.New(-50, "USD")}}
		}
		group := domain.TxnLink{TxnIDs: []string{"a", "b"}}
		all := []domain.Transaction{tx, {ID: "b", Amount: money.New(-50, "USD")}}
		x := BuildIndex([]domain.Transaction{tx}, []domain.Account{{ID: "bank", Name: "Bank"}, {ID: "other", Name: "Other"}}, nil, nil, nil, nil)
		x.Refund["a"], x.BillMatched["a"], x.EventNames["a"] = true, true, "Trip"
		ctx := RowContext{Index: x, Labels: labels(), Groups: map[string]domain.TxnLink{"a": group}, GroupTransactions: Transactions(all), Rates: currency.Rates{Base: "USD"}}
		got := DeriveRow(ctx, RowInput{ID: "a", Amount: tx.Amount, Date: tx.Date, Automatic: true, Duplicate: true, Spike: "spike"})
		if got.GroupTotal != "-150 USD" || got.GroupSize != 2 || !got.IsDebtPayment || !got.IsRefund || !got.IsBillMatched || got.EventName != "Trip" || !got.AutoCategory {
			t.Fatalf("%+v", got)
		}
		if transfer && got.Category != "Bank \u2192 Other" {
			t.Fatal(got.Category)
		}
		if !transfer && !strings.HasPrefix(got.Category, "splitEditor.cellLabel:") {
			t.Fatal(got.Category)
		}
		all[1].Amount = money.New(math.MinInt64, "USD")
		ctx.GroupTransactions = Transactions(all)
		if got := DeriveRow(ctx, RowInput{ID: "a"}); got.GroupTotal != "" || strings.Contains(got.InfoText, "txnlinks.groupBadgeTitle") {
			t.Fatal(got)
		}
	}
}

func FuzzGroupSumReconciles(f *testing.F) {
	f.Add(int64(-200), int64(100))
	f.Add(int64(math.MaxInt64), int64(1))
	f.Fuzz(func(t *testing.T, a, b int64) {
		rows := []domain.Transaction{{Amount: money.New(a, "USD")}, {Amount: money.New(b, "USD")}}
		got, err := GroupSum(rows, currency.Rates{})
		want, e := money.New(a, "USD").Add(money.New(b, "USD"))
		if !reflect.DeepEqual(got, want) || (err != nil) != (e != nil) {
			t.Fatalf("%+v %v / %+v %v", got, err, want, e)
		}
	})
}

func TestTransferAndReviewLabels(t *testing.T) {
	for _, tt := range []struct {
		category string
		outgoing bool
		want     string
	}{
		{"From \u2192 To", true, "From \u2192 To"}, {"Transfer", true, "r4c.transferTo:To"}, {"Transfer", false, "r4c.transferFrom:To"},
	} {
		if got := TransferDisplay(tt.category, "To", tt.outgoing, translate); got != tt.want {
			t.Fatal(got)
		}
	}
	if TransferTooltip("detail", "To", translate) != "detail" || TransferTooltip("", "To", translate) != "r4c.transferCounterparty:To" {
		t.Fatal("tooltip")
	}
	for _, queued := range []bool{false, true} {
		for _, transfer := range []bool{false, true} {
			want := ""
			if queued && !transfer {
				want = "acctxn.legendNeedsReview"
			}
			if ReviewLabel(queued, transfer, translate) != want {
				t.Fatal(queued, transfer)
			}
		}
	}
}

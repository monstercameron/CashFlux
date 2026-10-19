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
	"github.com/monstercameron/CashFlux/internal/payeealias"
	"github.com/monstercameron/CashFlux/internal/similartxns"
)

func TestAccountPreview(t *testing.T) {
	row, accounts := example()
	for _, tc := range []struct {
		id, key string
		lines   int
	}{{"", "", 0}, {"a", "", 0}, {"b", "", 1}, {"c", "transactions.accountMoveCurrency", 0}, {"missing", "transactions.accountMoveRefused", 0}} {
		v := AccountPreview(row, tc.id, accounts, nil)
		if !v.Show || v.Refusal.Key != tc.key || len(v.Lines) != tc.lines {
			t.Fatalf("%s: %+v", tc.id, v)
		}
	}
	row.TransferAccountID = "b"
	if v := AccountPreview(row, "b", accounts, nil); v.Refusal.Key != "transactions.accountMoveSelf" {
		t.Fatal(v)
	}
	row.Cleared = true
	accounts[0].Reconciliations = []domain.Reconciliation{{At: row.Date.Add(time.Hour)}}
	partner := row
	partner.ID = "p"
	partner.AccountID = "b"
	partner.TransferAccountID = "a"
	partner.Amount = money.New(100, "USD")
	v := AccountPreview(row, "debt", accounts, []domain.Transaction{row, partner})
	if len(v.Lines) != 3 || v.Lines[1].Key != "transactions.accountMovePartner" || v.Lines[2].Key != "transactions.accountMoveReconciled" {
		t.Fatalf("paired reconciled move: %+v", v)
	}
	if AccountPreview(row, "", nil, nil).Show {
		t.Fatal("nil accounts")
	}
	// Archived current choices remain present; same-name types stay distinguishable.
	accounts = []domain.Account{{ID: "a", Name: "Same", Type: domain.AccountType("checking")}, {ID: "b", Name: " same ", Type: domain.AccountType("savings")}, {ID: "x", Name: "Hidden", Archived: true}, {ID: "old", Name: "Old", Archived: true}, {ID: "other", Name: "Same", Type: ""}}
	v = AccountPreview(row, "old", accounts, nil)
	if len(v.Options) != 4 {
		t.Fatal(v.Options)
	}
	labels := map[string]string{}
	for _, o := range v.Options {
		labels[o.Value] = o.Label
	}
	if labels["a"] != "Same · Checking" || labels["other"] != "Same · Other" || labels["old"] != "Old" {
		t.Fatal(labels)
	}
}

func TestMovementPreview(t *testing.T) {
	row, accounts := example()
	partner := row
	partner.ID = "p"
	partner.AccountID = "b"
	partner.Amount = money.New(100, "USD")
	for _, tc := range []struct {
		name, id    string
		rows        []domain.Transaction
		key, effect string
	}{
		{"unselected", "", nil, "", ""},
		{"none", "b", nil, "transactions.classifyPairNone", "transactions.classifyEffectNeutral"},
		{"exact", "b", []domain.Transaction{partner}, "transactions.classifyPairFound", "transactions.classifyEffectNeutral"},
		{"unknown", "missing", nil, "sol1.transferUnavailable", ""},
		{"debt", "debt", nil, "transactions.classifyPairNone", "transactions.classifyEffectDebt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := MovementPreview(row, tc.id, true, accounts, tc.rows)
			if !v.Show || v.Pair.Key != tc.key || v.Effect.Key != tc.effect || v.ShowSign != (tc.id != "") {
				t.Fatal(v)
			}
			if tc.id == "debt" && v.DebtAccount != "Card" {
				t.Fatal(v)
			}
		})
	}
	partner.Amount = money.New(99, "USD")
	v := MovementPreview(row, "b", false, accounts, []domain.Transaction{partner})
	if v.Pair.Key != "transactions.classifyPairNear" || v.Pair.Args[1] != "$0.99" {
		t.Fatal(v)
	}
	if MovementPreview(row, "", false, nil, nil).Show {
		t.Fatal("empty accounts")
	}
}

func TestHeadingAndDraftDecisions(t *testing.T) {
	for _, tc := range []struct{ desc, payee, title, subtitle string }{{" bank ", " Shop ", "bank", "Shop"}, {"", " Shop ", "Shop", ""}, {"Shop", "Shop", "Shop", ""}, {"", "", "", ""}} {
		a, b := Heading(domain.Transaction{Desc: tc.desc, Payee: tc.payee})
		if a != tc.title || b != tc.subtitle {
			t.Fatal(tc, a, b)
		}
	}
	for _, tc := range []struct {
		row  domain.Transaction
		want string
	}{{domain.Transaction{Payee: " p ", Desc: "d"}, "p"}, {domain.Transaction{Payee: " ", Desc: " d "}, "d"}, {domain.Transaction{}, ""}} {
		if RulePhrase(tc.row) != tc.want {
			t.Fatal(tc)
		}
	}
	cats := []domain.Category{{ID: "c", Name: "Food"}}
	for _, tc := range []struct {
		id, selected    string
		available, want bool
	}{{"c", "", true, true}, {"c", "c", true, false}, {"missing", "", true, false}, {"c", "", false, false}} {
		name, show := SuggestionName(cats, tc.id, tc.selected, tc.available)
		if show != tc.want || (show && name != "Food") {
			t.Fatal(tc, name, show)
		}
	}
	options, colors := Members([]domain.Member{{ID: "m", Name: "Cam", Color: "red"}})
	if !reflect.DeepEqual(options, []Option{{"m", "Cam"}}) || colors["m"] != "red" {
		t.Fatal(options, colors)
	}
	if opts, col := Members(nil); len(opts) != 0 || len(col) != 0 {
		t.Fatal(opts, col)
	}
	for _, tc := range []struct {
		old, newID string
		want       bool
	}{{"", "", false}, {"a", "a", false}, {"a", "", false}, {"a", "b", true}} {
		if CategoryChanged(domain.Transaction{CategoryID: tc.old}, domain.Transaction{CategoryID: tc.newID}) != tc.want {
			t.Fatal(tc)
		}
	}
}

func TestSimilarOffer(t *testing.T) {
	row, _ := example()
	row.Payee = ""
	row.Amount = money.New(-100, "USD")
	for _, n := range []int{0, 1, 5, 8} {
		candidates := make([]similartxns.Candidate, n)
		for i := range candidates {
			candidates[i] = similartxns.Candidate{Txn: row, AlreadyCategorized: i%2 == 0}
		}
		v := SimilarOffer(candidates, payeealias.NewResolver(nil))
		if len(v.Rows) != min(n, 5) || v.Remaining != max(0, n-5) {
			t.Fatal(v)
		}
		key := "similartxns.offer"
		if n == 1 {
			key = "similartxns.offerOne"
		}
		if v.Offer.Key != key {
			t.Fatal(v)
		}
		if n > 0 && (v.Rows[0].Name != "Bank" || v.Rows[0].Date != "Feb 29" || v.Rows[0].Amount != "($1.00)" || !v.Rows[0].AlreadyCategorized) {
			t.Fatal(v.Rows)
		}
	}
	row.Payee = "SHOP"
	v := SimilarOffer([]similartxns.Candidate{{Txn: row}}, nil)
	if v.Rows[0].Name == "bank" {
		t.Fatal(v)
	}
}

func TestMerchantContext(t *testing.T) {
	row, _ := example()
	row.Payee = "SHOP"
	now := row.Date
	history := make([]domain.Transaction, 0, 8)
	for _, amount := range []money.Money{money.New(-100, "USD"), money.New(-200, "EUR"), money.New(-300, "USD"), money.New(-math.MaxInt64, "MISSING")} {
		r := row
		r.Amount = amount
		history = append(history, r)
	}
	income := row
	income.Amount = money.New(100, "USD")
	history = append(history, income)
	transfer := row
	transfer.TransferAccountID = "b"
	history = append(history, transfer)
	other := row
	other.Payee = "Other"
	history = append(history, other)
	for _, tc := range []struct {
		name, base string
		rates      map[string]float64
		count      int
		show       bool
	}{
		{"missing FX", "USD", nil, 2, false}, {"mixed", "USD", map[string]float64{"EUR": 2}, 3, true}, {"default base", "", map[string]float64{"EUR": 2}, 3, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := MerchantContext(row, history, nil, tc.base, tc.rates, now, time.Monday)
			if v.Stats.Count != tc.count || v.Show != tc.show || !v.HasThis || v.ThisMagnitude != 100 || v.Base != "USD" {
				t.Fatal(v)
			}
		})
	}
	for _, tc := range []struct {
		name      string
		edit      func(*domain.Transaction)
		wantCount int
		has       bool
	}{
		{"transfer", func(r *domain.Transaction) { r.TransferAccountID = "b" }, 0, false},
		{"empty", func(r *domain.Transaction) { r.Payee = " "; r.Desc = "" }, 0, false},
		{"fallback", func(r *domain.Transaction) { r.Payee = ""; r.Desc = "SHOP" }, 2, true},
		{"missing current FX", func(r *domain.Transaction) { r.Amount.Currency = "MISSING" }, 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := row
			tc.edit(&r)
			v := MerchantContext(r, history, nil, "USD", nil, now, time.Sunday)
			if v.Stats.Count != tc.wantCount || v.HasThis != tc.has {
				t.Fatal(v)
			}
		})
	}
	minimum := row
	minimum.Amount = money.New(math.MinInt64, "USD")
	boundary := MerchantContext(minimum, []domain.Transaction{minimum}, nil, "USD", nil, now, time.Monday)
	if !boundary.HasThis || boundary.ThisMagnitude != math.MinInt64 || boundary.Stats.Count != 1 {
		t.Fatal(boundary)
	}

	// The resolver matches aliases and uses the row's description only when payee is empty.
	row.Payee = ""
	row.Desc = "raw"
	r := row
	v := MerchantContext(row, []domain.Transaction{r, r, r}, payeealias.NewResolver([]domain.PayeeAlias{{RawPayee: "raw", Display: "Shop"}}), "USD", nil, now, time.Monday)
	if !v.Show || v.Merchant != "Shop" {
		t.Fatal(v)
	}
}

func TestIndependentWrites(t *testing.T) {
	row, _ := example()
	row.Note = "keep"
	for _, tc := range []struct {
		splits []domain.CategorySplit
		key    string
	}{{nil, "splitEditor.cleared"}, {[]domain.CategorySplit{{CategoryID: "c", Amount: money.New(-100, "USD")}}, "splitEditor.saved"}} {
		got, key := SplitUpdate(row, domain.Transaction{Splits: tc.splits})
		if key != tc.key || got.Note != "keep" || len(got.Splits) != len(tc.splits) {
			t.Fatal(got, key)
		}
		if len(got.Splits) > 0 {
			got.Splits[0].CategoryID = "changed"
			if tc.splits[0].CategoryID == "changed" {
				t.Fatal("aliased splits")
			}
		}
	}
	now := time.Unix(1, 2)
	for _, data := range [][]byte{nil, []byte("receipt")} {
		a := ReceiptArtifact("id", "receipt.png", "image/png", data, now)
		if a.ID != "id" || a.Name != "receipt.png" || a.Kind != "image" || a.MIME != "image/png" || a.Size != len(data) || !a.CreatedAt.Equal(now) || !reflect.DeepEqual(a.Bytes, data) {
			t.Fatal(a)
		}
	}
	original := row
	original.TransferAccountID = "b"
	for _, tc := range []struct {
		old, new domain.Transaction
		ids      []string
		want     bool
	}{{row, row, nil, false}, {original, original, []string{"t", "p"}, false}, {original, row, []string{"t"}, false}, {original, row, []string{"t", "p"}, true}} {
		id, confirm := PartnerRemoval(tc.old, tc.new, tc.ids)
		if confirm != tc.want || (confirm && id != "p") {
			t.Fatal(tc, id, confirm)
		}
	}
	for _, payee := range []string{"SHOP", ""} {
		row.Payee = payee
		msg := PartnerPrompt(row, []domain.Account{{ID: "a", Name: "Checking"}})
		if msg.Key != "transactions.deleteConfirmDetail" || msg.Args[1] != "2024-02-29" || msg.Args[3] != "Checking" {
			t.Fatal(msg)
		}
	}
}

func FuzzToggleOwner(f *testing.F) {
	f.Add("a,b", "a")
	f.Add("", "x")
	f.Fuzz(func(t *testing.T, list, owner string) {
		var owners []string
		if list != "" {
			owners = strings.Split(list, ",")
		}
		before := append([]string(nil), owners...)
		got := ToggleOwner(owners, owner)
		if !reflect.DeepEqual(before, owners) {
			t.Fatal("mutated input")
		}
		count := 0
		for _, v := range owners {
			if v == owner {
				count++
			}
		}
		if count == 0 && len(got) != len(owners)+1 || count > 0 && len(got) != len(owners)-1 {
			t.Fatal("wrong size")
		}
	})
}

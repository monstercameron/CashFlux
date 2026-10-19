// SPDX-License-Identifier: MIT

package txnlink

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
)

func TestBestSubscription(t *testing.T) {
	usd := currency.Rates{Base: "USD"}
	fx := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	tests := []struct {
		name, payee, desc, code string
		amount                  int64
		candidates              []subscriptions.Subscription
		rates                   currency.Rates
		want                    string
	}{
		{"nil", "HOA dues", "", "USD", -4000, nil, usd, ""},
		{"zero", "HOA dues", "", "USD", 0, []subscriptions.Subscription{{Name: "HOA dues", Amount: 0}}, usd, ""},
		{"no merchant", "  ", " ", "USD", -4000, []subscriptions.Subscription{{Name: "HOA dues", Amount: 4000}}, usd, ""},
		{"closest", " hoa dues ", "", "USD", -4000, []subscriptions.Subscription{{Name: "HOA dues", Amount: 4399}, {Name: "HOA dues", Amount: 4000}, {Name: "HOA dues", Amount: 4500}}, usd, "HOA dues"},
		{"description", "unrelated", " HOA DUES ", "USD", 4000, []subscriptions.Subscription{{Name: "HOA dues", Amount: 4000}}, usd, "HOA dues"},
		{"outside", "HOA dues", "", "USD", -4000, []subscriptions.Subscription{{Name: "HOA dues", Amount: 4401}}, usd, ""},
		{"boundary", "HOA dues", "", "USD", -4000, []subscriptions.Subscription{{Name: "HOA dues", Amount: 4400}}, usd, "HOA dues"},
		{"tiny exact", "X", "", "USD", -1, []subscriptions.Subscription{{Name: "X", Amount: 1}}, usd, "X"},
		{"tiny no tolerance", "X", "", "USD", -9, []subscriptions.Subscription{{Name: "X", Amount: 10}}, usd, ""},
		{"negative candidate", "X", "", "USD", -100, []subscriptions.Subscription{{Name: "X", Amount: -100}}, usd, ""},
		{"unrelated", "X", "", "USD", -100, []subscriptions.Subscription{{Name: "Y", Amount: 100}}, usd, ""},
		{"stable ties", "X", "", "USD", -100, []subscriptions.Subscription{{Name: " x ", Amount: 99}, {Name: "X", Amount: 101}}, usd, " x "},
		{"FX", "X", "", "EUR", -100, []subscriptions.Subscription{{Name: "X", Amount: 200}}, fx, "X"},
		{"missing FX", "X", "", "EUR", -100, []subscriptions.Subscription{{Name: "X", Amount: 100}}, usd, ""},
		{"overflow abs", "X", "", "USD", math.MinInt64, []subscriptions.Subscription{{Name: "X", Amount: math.MaxInt64}}, usd, ""},
		{"overflow FX", "X", "", "EUR", -math.MaxInt64, []subscriptions.Subscription{{Name: "X", Amount: math.MaxInt64}}, fx, ""},
		{"maximum exact", "X", "", "USD", math.MaxInt64, []subscriptions.Subscription{{Name: "X", Amount: 0}, {Name: "X", Amount: math.MaxInt64}}, usd, "X"},
		{"maximum difference", "X", "", "USD", 1, []subscriptions.Subscription{{Name: "X", Amount: math.MaxInt64}}, usd, ""},
		{"normalized currency", "X", "", " usd ", -100, []subscriptions.Subscription{{Name: "X", Amount: 100}}, usd, "X"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txn := domain.Transaction{Payee: tt.payee, Desc: tt.desc, Amount: money.Money{Amount: tt.amount, Currency: tt.code}}
			if got := BestSubscription(txn, tt.candidates, tt.rates); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSubscriptionNames(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   []subscriptions.Subscription
		want []string
	}{
		{"nil", nil, nil}, {"empty", []subscriptions.Subscription{}, nil},
		{"stable trim and exact duplicates", []subscriptions.Subscription{{Name: " B "}, {Name: ""}, {Name: "  "}, {Name: "A"}, {Name: "B"}, {Name: "a"}}, []string{"B", "A", "a"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := SubscriptionNames(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
	large := make([]subscriptions.Subscription, 50000)
	for i := range large {
		large[i].Name = " repeated "
	}
	if got := SubscriptionNames(large); !reflect.DeepEqual(got, []string{"repeated"}) {
		t.Fatal(got)
	}
}

func TestFilterSubscriptionKinds(t *testing.T) {
	for _, tt := range []struct {
		name      string
		candidate string
		txns      []domain.Transaction
		accounts  []domain.Account
		cats      map[string]string
		confirmed map[string]bool
		ignored   []domain.SubscriptionIgnore
		want      int
	}{
		{name: "blank", candidate: "  "},
		{name: "unmatched", candidate: "HOA dues"},
		{name: "name", candidate: "Music service", txns: []domain.Transaction{{Desc: " MUSIC SERVICE "}}, want: 1},
		{name: "category", candidate: "Club", txns: []domain.Transaction{{Desc: "Other"}, {Desc: "Club", CategoryID: "m"}}, cats: map[string]string{"m": "Membership"}, want: 1},
		{name: "confirmation", candidate: "Club", txns: []domain.Transaction{{Desc: "Club"}}, confirmed: map[string]bool{"club": true}, want: 1},
		{name: "nil maps", candidate: "Zorp", txns: []domain.Transaction{{Desc: "Zorp"}}},
		{name: "habitual", candidate: "Gas", txns: []domain.Transaction{{Desc: "Gas"}}, confirmed: map[string]bool{"gas": true}},
		{name: "ignored", candidate: "HOA dues", txns: []domain.Transaction{{Desc: "HOA dues"}}, ignored: []domain.SubscriptionIgnore{{SubName: " HOA DUES "}}},
		{name: "liability", candidate: "HOA dues", txns: []domain.Transaction{{Desc: "HOA dues", AccountID: "debt"}}, accounts: []domain.Account{{ID: "debt", Class: domain.ClassLiability}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterSubscriptionKinds([]subscriptions.Subscription{{Name: tt.candidate}}, tt.txns, tt.txns, tt.accounts, tt.cats, tt.confirmed, tt.ignored)
			if len(got) != tt.want {
				t.Fatalf("got %#v, want %d items", got, tt.want)
			}
		})
	}
	if got := FilterSubscriptionKinds(nil, nil, nil, nil, nil, nil, nil); len(got) != 0 {
		t.Fatal(got)
	}
	in := []subscriptions.Subscription{{Name: "Second membership"}, {Name: "First membership"}}
	txns := []domain.Transaction{{Desc: in[0].Name}, {Desc: in[1].Name}}
	if got := FilterSubscriptionKinds(in, txns, txns, nil, nil, nil, nil); !reflect.DeepEqual(got, in) {
		t.Fatalf("order changed: %#v", got)
	}
	// Liability signals from excluded detection rows still apply to a candidate.
	got := FilterSubscriptionKinds(in[:1], txns[:1], []domain.Transaction{{Desc: in[0].Name, Payee: "Chase"}}, nil, nil, nil, nil)
	if len(got) != 0 {
		t.Fatal(got)
	}
}

func detectionInput(t *testing.T) Input {
	t.Helper()
	in := Input{Accounts: []domain.Account{{ID: "bank", Name: "Checking", Type: domain.TypeChecking}, {ID: "debt", Name: "Card", Class: domain.ClassLiability}, {ID: "old", Name: "Archived", Class: domain.ClassLiability, Archived: true}}, Categories: []domain.Category{{ID: "membership", Name: "Membership"}}}
	for i, day := range []string{"2026-01-31", "2026-02-28", "2026-03-31"} {
		date, err := dateutil.ParseDate(day)
		if err != nil {
			t.Fatal(err)
		}
		in.Transactions = append(in.Transactions, domain.Transaction{ID: []string{"one", "two", "three"}[i], Desc: "HOA dues", AccountID: "bank", CategoryID: "membership", Date: date, Amount: money.New(-4000, "USD")})
	}
	return in
}

func TestPrepare(t *testing.T) {
	for _, tt := range []struct {
		name    string
		modify  func(*Input)
		want    int
		initial string
	}{
		{"default", func(*Input) {}, 1, "HOA dues"},
		{"base", func(in *Input) { in.BaseCurrency = "USD" }, 1, "HOA dues"},
		{"ignore category", func(in *Input) { in.IgnoredCategoryIDs = []string{"membership"} }, 0, ""},
		{"ignore type", func(in *Input) { in.IgnoredAccountTypes = []string{"checking"} }, 0, ""},
		{"higher threshold", func(in *Input) { in.MinOccurrences = 4 }, 0, ""},
		{"foreign missing FX", func(in *Input) { in.Transactions[0].Amount = money.New(-4000, "EUR") }, 0, ""},
		{"foreign FX", func(in *Input) {
			in.Transactions[0].Amount = money.New(-2000, "EUR")
			in.FXRates = map[string]float64{"EUR": 2}
		}, 1, "HOA dues"},
		{"existing absent", func(in *Input) { in.Transactions[0].SubscriptionName = "Old link" }, 2, "Old link"},
		{"existing present", func(in *Input) {
			in.Transactions[0].SubscriptionName = "HOA dues"
			in.Transactions[0].BillAccountID = "debt"
		}, 1, "HOA dues"},
		{"unrepresentable magnitude", func(in *Input) { in.Transactions[0].Amount.Amount = math.MinInt64 }, 0, ""},
		{"ignored merchant", func(in *Input) { in.Ignored = []domain.SubscriptionIgnore{{SubName: "hoa dues"}} }, 0, ""},
		{"income ignored", func(in *Input) { in.Transactions[0].Amount.Amount = 4000 }, 1, "HOA dues"},
		{"timezone dates", func(in *Input) {
			for i := range in.Transactions {
				in.Transactions[i].Date = in.Transactions[i].Date.In(time.FixedZone("west", -7*3600))
			}
		}, 1, "HOA dues"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := detectionInput(t)
			tt.modify(&in)
			before := append([]domain.Transaction(nil), in.Transactions...)
			got := Prepare(in, "one")
			if !got.Found || len(got.Subscriptions) != tt.want || got.Initial.SubscriptionName != tt.initial {
				t.Fatalf("got %#v", got)
			}
			if !reflect.DeepEqual(got.Bills, []Option{{Value: "debt", Label: "Card"}}) || got.AccountNames["old"] != "Archived" {
				t.Fatalf("accounts: %#v", got)
			}
			if got.Initial.BillAccountID != in.Transactions[0].BillAccountID {
				t.Fatal("bill initial lost")
			}
			if !reflect.DeepEqual(before, in.Transactions) {
				t.Fatal("mutated input")
			}
			if tt.name == "existing absent" && got.Subscriptions[0].Value != "Old link" {
				t.Fatal("existing link not first")
			}
		})
	}
	for _, in := range []Input{{}, {Transactions: []domain.Transaction{{ID: "other"}}}} {
		if got := Prepare(in, "missing"); got.Found || got.Transaction.ID != "" {
			t.Fatal(got)
		}
	}
	// First match wins, including duplicate IDs, just as the original lookup did.
	in := Input{Transactions: []domain.Transaction{{ID: "x", Desc: "first"}, {ID: "x", Desc: "second"}}}
	if got := Prepare(in, "x"); got.Transaction.Desc != "first" {
		t.Fatal(got)
	}
}

func TestDraftEvaluateAndNotice(t *testing.T) {
	for _, tt := range []struct {
		name                                string
		txn                                 domain.Transaction
		draft                               Draft
		changed, linked, clearing, remember bool
		save, hint, testID, notice          string
	}{
		{name: "empty", save: "txnlink.save", hint: "txnlink.nothingToSave", testID: "txnlink-nothing", notice: "txnlink.cleared"},
		{name: "toggle without bill", draft: Draft{AutoLink: true}, save: "txnlink.save", hint: "txnlink.nothingToSave", testID: "txnlink-nothing", notice: "txnlink.cleared"},
		{name: "bill", draft: Draft{BillAccountID: "debt"}, changed: true, linked: true, save: "txnlink.save", notice: "txnlink.saved"},
		{name: "subscription", draft: Draft{SubscriptionName: "Sub"}, changed: true, linked: true, save: "txnlink.save", notice: "txnlink.saved"},
		{name: "both", draft: Draft{BillAccountID: "debt", SubscriptionName: "Sub", AutoLink: true}, changed: true, linked: true, remember: true, save: "txnlink.save", notice: "txnlink.saved"},
		{name: "unchanged", txn: domain.Transaction{BillAccountID: "debt", SubscriptionName: "Sub"}, draft: Draft{BillAccountID: "debt", SubscriptionName: "Sub"}, linked: true, save: "txnlink.save", hint: "txnlink.nothingToSave", testID: "txnlink-nothing", notice: "txnlink.saved"},
		{name: "rule only", txn: domain.Transaction{BillAccountID: "debt"}, draft: Draft{BillAccountID: "debt", AutoLink: true}, changed: true, linked: true, remember: true, save: "txnlink.save", notice: "txnlink.saved"},
		{name: "remove all", txn: domain.Transaction{BillAccountID: "debt", SubscriptionName: "Sub"}, changed: true, clearing: true, save: "txnlink.saveRemove", hint: "txnlink.clearHint", testID: "txnlink-clearhint", notice: "txnlink.cleared"},
		{name: "remove bill keep sub", txn: domain.Transaction{BillAccountID: "debt", SubscriptionName: "Sub"}, draft: Draft{SubscriptionName: "Sub"}, changed: true, linked: true, save: "txnlink.save", notice: "txnlink.saved"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			original := tt.txn
			got := tt.draft.Evaluate(tt.txn)
			if got.HasBill != (tt.draft.BillAccountID != "") || got.HasSubscription != (tt.draft.SubscriptionName != "") {
				t.Fatal("preview or remember visibility disagrees with the draft")
			}
			if got.Changed != tt.changed || got.Linked != tt.linked || got.Clearing != tt.clearing || got.Remember != tt.remember || got.SaveKey != tt.save || got.HintKey != tt.hint || got.HintTestID != tt.testID || got.NoticeKey(false) != tt.notice || got.NoticeKey(true) != "txnlink.savedWithRule" {
				t.Fatalf("got %#v", got)
			}
			if !reflect.DeepEqual(tt.txn, original) {
				t.Fatal("mutated original")
			}
			if got.Transaction.BillAccountID != tt.draft.BillAccountID || got.Transaction.SubscriptionName != tt.draft.SubscriptionName {
				t.Fatal("wrong transaction")
			}
		})
	}
}

func TestDraftTransitions(t *testing.T) {
	for _, initial := range []Draft{{}, {BillAccountID: "bill", SubscriptionName: "sub", AutoLink: true}} {
		for _, value := range []string{"", "new", "  spaced  ", strings.Repeat("x", 50000)} {
			bill := initial.WithBill(value)
			if bill.BillAccountID != value || bill.SubscriptionName != initial.SubscriptionName || bill.AutoLink != initial.AutoLink {
				t.Fatal("bill selection changed another choice")
			}
			sub := initial.WithSubscription(value)
			if sub.SubscriptionName != value || sub.BillAccountID != initial.BillAccountID || sub.AutoLink != initial.AutoLink {
				t.Fatal("subscription selection changed another choice")
			}
		}
		toggled := initial.ToggleAutoLink()
		if toggled.AutoLink == initial.AutoLink || toggled.BillAccountID != initial.BillAccountID || toggled.SubscriptionName != initial.SubscriptionName {
			t.Fatal("rule selection changed a link")
		}
		if toggled.ToggleAutoLink() != initial {
			t.Fatal("two toggles did not restore the original draft")
		}
	}
}

func TestPrepareLargeHistory(t *testing.T) {
	in := Input{Transactions: make([]domain.Transaction, 50000)}
	in.Transactions[len(in.Transactions)-1] = domain.Transaction{ID: "last", BillAccountID: "debt", SubscriptionName: "Existing"}
	m := Prepare(in, "last")
	if !m.Found || m.Initial.BillAccountID != "debt" || m.Initial.SubscriptionName != "Existing" || len(m.Subscriptions) != 1 {
		t.Fatal("large history lost the selected transaction")
	}
}

func TestBillRule(t *testing.T) {
	for _, tt := range []struct {
		name, payee, desc string
		draft             Draft
		existing          []rules.Rule
		want              bool
		phrase            string
	}{
		{name: "disabled", payee: "X", draft: Draft{BillAccountID: "debt"}},
		{name: "no bill", payee: "X", draft: Draft{AutoLink: true}},
		{name: "blank", payee: " ", desc: "Fallback", draft: Draft{BillAccountID: "debt", AutoLink: true}},
		{name: "none", draft: Draft{BillAccountID: "debt", AutoLink: true}},
		{name: "payee", payee: "  Merchant  ", desc: "Other", draft: Draft{BillAccountID: "debt", AutoLink: true}, want: true, phrase: "Merchant"},
		{name: "description", desc: "  Desc  ", draft: Draft{BillAccountID: "debt", AutoLink: true}, want: true, phrase: "Desc"},
		{name: "duplicate", payee: "Merchant", draft: Draft{BillAccountID: "debt", AutoLink: true}, existing: []rules.Rule{{Match: " MERCHANT ", SetBillAccountID: "debt"}}},
		{name: "other account", payee: "Merchant", draft: Draft{BillAccountID: "debt", AutoLink: true}, existing: []rules.Rule{{Match: "Merchant", SetBillAccountID: "other"}}, want: true, phrase: "Merchant"},
		{name: "other phrase", payee: "Merchant", draft: Draft{BillAccountID: "debt", AutoLink: true}, existing: []rules.Rule{{Match: "Other", SetBillAccountID: "debt"}}, want: true, phrase: "Merchant"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := BillRule(tt.existing, domain.Transaction{Payee: tt.payee, Desc: tt.desc}, tt.draft, "new-id", 7)
			if ok != tt.want {
				t.Fatalf("got %#v, %v", got, ok)
			}
			if ok && (got.ID != "new-id" || got.Match != tt.phrase || got.SetBillAccountID != "debt" || got.Order != 7) {
				t.Fatal(got)
			}
			if ok {
				if _, again := BillRule(append(tt.existing, got), domain.Transaction{Payee: tt.payee, Desc: tt.desc}, tt.draft, "new-id-2", 8); again {
					t.Fatal("duplicate inserted")
				}
			}
		})
	}
}

func TestDescriptionAndMerchant(t *testing.T) {
	for _, tt := range []struct{ payee, desc, wantDesc, wantMerchant string }{
		{"", "", "Fallback", "Fallback"}, {"Payee", "", "Payee", "Payee"}, {"Payee", "Desc", "Desc", "Payee"},
		{"", " Desc ", " Desc ", "Desc"}, {" ", "Desc", "Desc", "Fallback"}, {"Payee", " ", " ", "Payee"},
	} {
		txn := domain.Transaction{Payee: tt.payee, Desc: tt.desc}
		if got := Description(txn, "Fallback"); got != tt.wantDesc {
			t.Errorf("Description(%#v)=%q", txn, got)
		}
		if got := Merchant(txn, "Fallback"); got != tt.wantMerchant {
			t.Errorf("Merchant(%#v)=%q", txn, got)
		}
	}
}

func TestSummarize(t *testing.T) {
	date := time.Date(2024, time.February, 29, 23, 59, 0, 0, time.FixedZone("west", -8*3600))
	for _, tt := range []struct {
		name                       string
		amount                     int64
		code                       string
		style                      prefs.DateStyle
		names                      map[string]string
		wantDate, wantAmount, tone string
	}{
		{"negative", -1234, "USD", prefs.DateISO, map[string]string{"a": "Checking"}, "2024-02-29 · Checking", "($12.34)", "text-down"},
		{"positive", 1234, "USD", prefs.DateUS, nil, "02/29/2024", "$12.34", "text-up"},
		{"zero", 0, "JPY", prefs.DateEU, nil, "29/02/2024", "¥0", ""},
		{"month end", 1, "JPY", prefs.DateLong, nil, "Feb 29, 2024", "¥1", "text-up"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			txn := domain.Transaction{Desc: "Description", AccountID: "a", Date: date, Amount: money.New(tt.amount, tt.code)}
			got := Summarize(txn, tt.names, prefs.Prefs{DateStyle: tt.style}, "Fallback")
			if got.Description != "Description" || got.AccountLine != tt.wantDate || got.Amount != tt.wantAmount || got.Tone != tt.tone {
				t.Fatalf("got %#v", got)
			}
		})
	}
}

func FuzzSubscriptionNamesStable(f *testing.F) {
	f.Add(" B ,A,B,, A ,a")
	f.Add("")
	f.Fuzz(func(t *testing.T, raw string) {
		parts := strings.Split(raw, ",")
		in := make([]subscriptions.Subscription, len(parts))
		var want []string
		seen := map[string]bool{}
		for i, name := range parts {
			in[i].Name = name
			name = strings.TrimSpace(name)
			if name != "" && !seen[name] {
				want = append(want, name)
				seen[name] = true
			}
		}
		got := SubscriptionNames(in)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
		roundtrip := make([]subscriptions.Subscription, len(got))
		for i, name := range got {
			roundtrip[i].Name = name
		}
		if !reflect.DeepEqual(SubscriptionNames(roundtrip), got) {
			t.Fatal("not idempotent")
		}
	})
}

func FuzzDraftRoundTrip(f *testing.F) {
	f.Add("bill", "subscription", "next", "", false)
	f.Add("", "", "", "", true)
	f.Fuzz(func(t *testing.T, bill, sub, nextBill, nextSub string, remember bool) {
		txn := domain.Transaction{ID: "keep", BillAccountID: bill, SubscriptionName: sub, Amount: money.New(math.MaxInt64, "USD")}
		d := Draft{BillAccountID: nextBill, SubscriptionName: nextSub, AutoLink: remember}
		decision := d.Evaluate(txn)
		if got := (Draft{BillAccountID: bill, SubscriptionName: sub}).Evaluate(decision.Transaction).Transaction; !reflect.DeepEqual(got, txn) {
			t.Fatal("roundtrip lost data")
		}
		d.AutoLink = false
		if d.Evaluate(decision.Transaction).Changed {
			t.Fatal("saved draft still changed")
		}
	})
}

func FuzzBestSubscriptionMagnitude(f *testing.F) {
	f.Add(int64(-4000), int64(4000))
	f.Add(int64(math.MinInt64), int64(math.MaxInt64))
	f.Add(int64(math.MaxInt64), int64(0))
	f.Fuzz(func(t *testing.T, amount, candidate int64) {
		txn := domain.Transaction{Payee: "X", Amount: money.New(amount, "USD")}
		got := BestSubscription(txn, []subscriptions.Subscription{{Name: "X", Amount: candidate}}, currency.Rates{Base: "USD"})
		magnitude, err := money.Abs(amount)
		if err != nil || magnitude == 0 || candidate < 0 {
			if got != "" {
				t.Fatal("invalid magnitude matched")
			}
			return
		}
		diff, _ := money.Sub(candidate, magnitude)
		diff, _ = money.Abs(diff)
		if (got == "X") != (diff <= magnitude/10) {
			t.Fatalf("amount=%d candidate=%d result=%q", amount, candidate, got)
		}
	})
}

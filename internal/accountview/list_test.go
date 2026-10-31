// SPDX-License-Identifier: MIT

package accountview

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/liquidity"
)

func TestStaleCollapsesOnlyPastHalf(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	fresh := func(id string) domain.Account {
		return domain.Account{ID: id, Type: domain.TypeChecking, BalanceAsOf: now}
	}
	stale := func(id string) domain.Account {
		return domain.Account{ID: id, Type: domain.TypeChecking, BalanceAsOf: now.AddDate(0, 0, -400)}
	}
	cases := []struct {
		name     string
		accounts []domain.Account
		want     StaleSummary
	}{
		{"empty", nil, StaleSummary{}},
		{"none stale", []domain.Account{fresh("a"), fresh("b")}, StaleSummary{}},
		{"exactly half is not collapsed", []domain.Account{stale("a"), fresh("b")}, StaleSummary{Count: 1}},
		{"majority collapses", []domain.Account{stale("a"), stale("b"), fresh("c")}, StaleSummary{Count: 2, Collapsed: true}},
		{"all stale", []domain.Account{stale("a")}, StaleSummary{Count: 1, Collapsed: true}},
		{"archived is never stale", []domain.Account{{ID: "x", Type: domain.TypeChecking, Archived: true}}, StaleSummary{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Stale(tc.accounts, freshness.DefaultWindows(), now); got != tc.want {
				t.Fatalf("Stale = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func listFixture() []domain.Account {
	return []domain.Account{
		{ID: "chk", Name: "Checking", Type: domain.TypeChecking, OwnerID: "m"},
		{ID: "sav", Name: "Savings", Type: domain.TypeSavings, OwnerID: domain.GroupOwnerID},
		{ID: "card", Name: "Card", Type: domain.TypeCreditCard, Class: domain.ClassLiability, OwnerID: "m"},
		{ID: "theirs", Name: "Theirs", Type: domain.TypeChecking, OwnerID: "other"},
		{ID: "old", Name: "Old", Type: domain.TypeChecking, OwnerID: "m", Archived: true},
		{ID: "oldcard", Name: "Old card", Type: domain.TypeCreditCard, Class: domain.ClassLiability, OwnerID: "m", Archived: true},
	}
}

func TestListResolvesClassViewAndScope(t *testing.T) {
	const services = "service"
	cases := []struct {
		name       string
		member     string
		class      string
		match      func(name, typ string) bool
		wantShown  []string
		wantClass  string
		wantActive int
		wantLiabs  int
		services   bool
		assetsOnly bool
	}{
		{name: "everyone sees all active", class: "", wantShown: []string{"chk", "sav", "card", "theirs"}, wantClass: "", wantActive: 4, wantLiabs: 1},
		{name: "member scope keeps household accounts", member: "m", class: "", wantShown: []string{"chk", "sav", "card"}, wantClass: "", wantActive: 3, wantLiabs: 1},
		{name: "assets view drops liabilities", member: "m", class: string(domain.ClassAsset), wantShown: []string{"chk", "sav"}, wantClass: string(domain.ClassAsset), wantActive: 3, wantLiabs: 1, assetsOnly: true},
		{name: "liabilities view", member: "m", class: string(domain.ClassLiability), wantShown: []string{"card"}, wantClass: string(domain.ClassLiability), wantActive: 3, wantLiabs: 1},
		{name: "no liabilities falls back to assets", member: "other", class: string(domain.ClassLiability), wantShown: []string{"sav", "theirs"}, wantClass: string(domain.ClassAsset), wantActive: 2, wantLiabs: 0, assetsOnly: true},
		{name: "services view shows no accounts", member: "m", class: services, wantShown: nil, wantClass: services, wantActive: 3, wantLiabs: 1, services: true},
		{name: "services view survives no liabilities", member: "other", class: services, wantShown: nil, wantClass: services, wantActive: 2, wantLiabs: 0, services: true},
		{name: "match narrows", member: "m", class: "", match: func(name, _ string) bool { return strings.HasPrefix(name, "S") }, wantShown: []string{"sav"}, wantClass: "", wantActive: 3, wantLiabs: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := List(ListInput{Accounts: listFixture(), MemberID: tc.member, Class: tc.class, ServicesClass: services, Match: tc.match})
			if shown := ids(got.Shown); !reflect.DeepEqual(shown, tc.wantShown) && (len(shown) != 0 || len(tc.wantShown) != 0) {
				t.Fatalf("Shown = %v, want %v", shown, tc.wantShown)
			}
			if got.Class != tc.wantClass || len(got.Active) != tc.wantActive || len(got.Liabilities) != tc.wantLiabs {
				t.Fatalf("got class %q active %d liabilities %d; want %q %d %d",
					got.Class, len(got.Active), len(got.Liabilities), tc.wantClass, tc.wantActive, tc.wantLiabs)
			}
			if got.InServices != tc.services || got.AssetsOnly != tc.assetsOnly {
				t.Fatalf("InServices %v AssetsOnly %v, want %v %v", got.InServices, got.AssetsOnly, tc.services, tc.assetsOnly)
			}
		})
	}
}

func TestArchivedIncludesLiabilitiesInStoredOrder(t *testing.T) {
	if got := ids(Archived(listFixture(), "m")); !reflect.DeepEqual(got, []string{"old", "oldcard"}) {
		t.Fatalf("Archived(m) = %v", got)
	}
	if got := Archived(listFixture(), "other"); len(got) != 0 {
		t.Fatalf("Archived(other) = %v, want none", ids(got))
	}
}

func TestFilterOptionsTypesForeignAndStale(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	staleAsOf := now.AddDate(0, 0, -400)
	accounts := []domain.Account{
		{ID: "a", Type: domain.TypeSavings, Currency: "USD", OwnerID: "m", BalanceAsOf: now},
		{ID: "b", Type: domain.TypeChecking, Currency: "EUR", OwnerID: "other", BalanceAsOf: staleAsOf},
		{ID: "c", Type: domain.TypeChecking, Currency: "USD", OwnerID: domain.GroupOwnerID, BalanceAsOf: now},
		{ID: "d", Type: domain.TypeCrypto, Currency: "BTC", OwnerID: "m", Archived: true},
		{ID: "e", Type: domain.TypeInvestment, Currency: "USD", OwnerID: "m", BalanceAsOf: staleAsOf},
	}
	windows := freshness.DefaultWindows()
	cases := []struct {
		name     string
		accounts []domain.Account
		member   string
		want     FilterChoices
	}{
		{"member scope hides others' and archived accounts", accounts, "m", FilterChoices{Types: []string{"checking", "investment", "savings"}, StaleCount: 1}},
		{"everyone sees foreign and stale of every owner but not archived", accounts, "", FilterChoices{Types: []string{"checking", "investment", "savings"}, HasForeign: true, StaleCount: 2}},
		{"single local account", accounts[:1], "", FilterChoices{Types: []string{"savings"}}},
		{"only archived foreign", accounts[3:4], "m", FilterChoices{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FilterOptions(tc.accounts, tc.member, "USD", windows, now); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("FilterOptions = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestStaleCountsActiveAccountsOnly(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	stale := now.AddDate(0, 0, -400)
	accounts := []domain.Account{
		{ID: "s1", Type: domain.TypeChecking, BalanceAsOf: stale},
		{ID: "f1", Type: domain.TypeChecking, BalanceAsOf: now},
		{ID: "x1", Type: domain.TypeChecking, Archived: true},
		{ID: "x2", Type: domain.TypeChecking, Archived: true},
	}
	if got := Stale(accounts, freshness.DefaultWindows(), now); got != (StaleSummary{Count: 1}) {
		t.Fatalf("archived accounts diluted the majority: %+v", got)
	}
	accounts = append(accounts, domain.Account{ID: "s2", Type: domain.TypeChecking, BalanceAsOf: stale})
	if got := Stale(accounts, freshness.DefaultWindows(), now); got != (StaleSummary{Count: 2, Collapsed: true}) {
		t.Fatalf("2 of 3 active stale should collapse: %+v", got)
	}
}

func TestLiquidityBucketsOrderAndVisibility(t *testing.T) {
	cases := []struct {
		name      string
		view      LiquidityView
		wantOrder []liquidity.Class
		visible   bool
	}{
		{"empty", LiquidityView{Totals: map[liquidity.Class]int64{}}, nil, false},
		{"single bucket hidden", LiquidityView{Totals: map[liquidity.Class]int64{liquidity.Held: 5}}, []liquidity.Class{liquidity.Held}, false},
		{"two buckets visible and ordered", LiquidityView{Totals: map[liquidity.Class]int64{liquidity.Held: 5, liquidity.Available: 9}},
			[]liquidity.Class{liquidity.Available, liquidity.Held}, true},
		{"incomplete single bucket visible", LiquidityView{Totals: map[liquidity.Class]int64{liquidity.Available: 1}, Incomplete: true},
			[]liquidity.Class{liquidity.Available}, true},
		{"all four", LiquidityView{Totals: map[liquidity.Class]int64{liquidity.Held: 1, liquidity.Investments: 2, liquidity.Restricted: 3, liquidity.Available: 4}},
			[]liquidity.Class{liquidity.Available, liquidity.Restricted, liquidity.Investments, liquidity.Held}, true},
		{"negative total still counts", LiquidityView{Totals: map[liquidity.Class]int64{liquidity.Available: -4, liquidity.Held: 1}},
			[]liquidity.Class{liquidity.Available, liquidity.Held}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var order []liquidity.Class
			for _, b := range tc.view.Buckets() {
				order = append(order, b.Class)
			}
			if !reflect.DeepEqual(order, tc.wantOrder) {
				t.Fatalf("Buckets = %v, want %v", order, tc.wantOrder)
			}
			if got := tc.view.Visible(); got != tc.visible {
				t.Fatalf("Visible = %v, want %v", got, tc.visible)
			}
		})
	}
}

func TestListWithoutAServicesClassNeverEntersTheServicesView(t *testing.T) {
	cases := []struct {
		name      string
		member    string
		class     string
		wantShown []string
		wantClass string
		assetOnly bool
	}{
		{"blank class with liabilities lists everything", "m", "", []string{"chk", "sav", "card"}, "", false},
		{"blank class without liabilities falls back to assets", "other", "", []string{"sav", "theirs"}, string(domain.ClassAsset), true},
		{"asset class", "m", string(domain.ClassAsset), []string{"chk", "sav"}, string(domain.ClassAsset), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := List(ListInput{Accounts: listFixture(), MemberID: tc.member, Class: tc.class})
			if got.InServices {
				t.Fatal("InServices = true with no ServicesClass configured")
			}
			if shown := ids(got.Shown); !reflect.DeepEqual(shown, tc.wantShown) {
				t.Fatalf("Shown = %v, want %v", shown, tc.wantShown)
			}
			if got.Class != tc.wantClass || got.AssetsOnly != tc.assetOnly {
				t.Fatalf("class %q assetsOnly %v, want %q %v", got.Class, got.AssetsOnly, tc.wantClass, tc.assetOnly)
			}
		})
	}
}

// SPDX-License-Identifier: MIT

package accountselect

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func transferAccounts() []domain.Account {
	return []domain.Account{
		{ID: "chk", Name: "Checking", Type: domain.TypeChecking, Currency: "USD"},
		{ID: "sav", Name: "Savings", Type: domain.TypeSavings, Currency: "USD"},
		{ID: "k401", Name: "401k", Type: domain.TypeRetirement, Currency: "USD"},
		{ID: "home", Name: "Condo", Type: domain.TypeProperty, Currency: "USD"},
		{ID: "car", Name: "Car", Type: domain.TypeVehicle, Currency: "USD"},
		{ID: "card", Name: "Visa", Type: domain.TypeCreditCard, Class: domain.ClassLiability, Currency: "USD"},
		{ID: "travel", Name: "Travel Card (EUR)", Type: domain.TypeDebit, Currency: "EUR"},
		{ID: "old", Name: "Old", Type: domain.TypeChecking, Currency: "USD", Archived: true},
	}
}

func candidateIDs(cs []TransferCandidate) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.ID)
	}
	return out
}

func TestTransferCandidatesApplyEligibilityRules(t *testing.T) {
	from, to := TransferCandidates(transferAccounts(), "", "")
	// Sources are liquid cash only; destinations skip valuation-only holdings; archived is never offered.
	if got, want := candidateIDs(from), []string{"chk", "sav", "travel"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("from = %v, want %v", got, want)
	}
	if got, want := candidateIDs(to), []string{"k401", "chk", "sav", "travel", "card"}; !reflect.DeepEqual(got, want) {
		// ForPicker sorts by name, case-insensitively.
		t.Fatalf("to = %v, want %v", got, want)
	}
}

func TestTransferCandidatesLabelsAndExclusions(t *testing.T) {
	from, to := TransferCandidates(transferAccounts(), "chk", "sav")
	if got, want := candidateIDs(from), []string{"chk", "travel"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("from excludes the destination: %v, want %v", got, want)
	}
	for _, c := range to {
		if c.ID == "chk" {
			t.Fatal("to must exclude the chosen source")
		}
	}
	if len(to) != 4 {
		t.Fatalf("to = %v", candidateIDs(to))
	}
	labels := map[string]string{}
	payments := map[string]bool{}
	for _, c := range append(append([]TransferCandidate{}, from...), to...) {
		labels[c.ID] = c.Label
		payments[c.ID] = c.LiabilityPayment
	}
	if labels["sav"] != "Savings (USD)" {
		t.Fatalf("label = %q", labels["sav"])
	}
	if labels["travel"] != "Travel Card (EUR)" {
		t.Fatalf("a name that already carries its currency is not annotated again: %q", labels["travel"])
	}
	if !payments["card"] || payments["sav"] {
		t.Fatalf("only the liability is a payment: %v", payments)
	}
}

func TestParseOptionalMinor(t *testing.T) {
	cases := []struct {
		in     string
		want   int64
		wantOK bool
	}{
		{"", 0, true}, {"  ", 0, true}, {"0", 0, true}, {"1.50", 150, true},
		{"-1", 0, false}, {"abc", 0, false},
	}
	for _, tc := range cases {
		got, ok := ParseOptionalMinor(tc.in, "USD")
		if got != tc.want || ok != tc.wantOK {
			t.Errorf("ParseOptionalMinor(%q) = %d, %v; want %d, %v", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestAmountOK(t *testing.T) {
	accounts := transferAccounts()
	cases := []struct {
		from, amount string
		want         bool
	}{
		{"chk", "10", true}, {"chk", " 0.01 ", true}, {"chk", "", false}, {"chk", "0", false},
		{"chk", "-5", false}, {"chk", "x", false}, {"", "10", true},
	}
	for _, tc := range cases {
		if got := AmountOK(accounts, tc.from, tc.amount); got != tc.want {
			t.Errorf("AmountOK(%q, %q) = %v, want %v", tc.from, tc.amount, got, tc.want)
		}
	}
}

func TestParseTransfer(t *testing.T) {
	accounts := transferAccounts()
	base := TransferInput{FromID: "chk", ToID: "sav", Amount: "25.50", Date: "2026-10-31", DefaultDesc: "Transfer"}
	t.Run("a complete form", func(t *testing.T) {
		in := base
		in.Desc = "  Rainy day  "
		got, err := ParseTransfer(accounts, in)
		if err != nil {
			t.Fatal(err)
		}
		if got.AmountMinor != 2550 || got.Desc != "Rainy day" || !got.Date.Equal(time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)) ||
			got.FromAccountID != "chk" || got.ToAccountID != "sav" || got.ReceivedMinor != 0 || got.FeeMinor != 0 {
			t.Fatalf("params = %+v", got)
		}
	})
	t.Run("blank description and date take the defaults", func(t *testing.T) {
		in := base
		in.Date = ""
		got, err := ParseTransfer(accounts, in)
		if err != nil {
			t.Fatal(err)
		}
		if got.Desc != "Transfer" || !got.Date.IsZero() {
			t.Fatalf("params = %+v", got)
		}
	})
	t.Run("received and fee use their own currencies", func(t *testing.T) {
		in := base
		in.ToID, in.Received, in.Fee = "travel", "20", "1.25"
		got, err := ParseTransfer(accounts, in)
		if err != nil {
			t.Fatal(err)
		}
		if got.ReceivedMinor != 2000 || got.FeeMinor != 125 {
			t.Fatalf("params = %+v", got)
		}
	})
	errCases := []struct {
		name   string
		mutate func(*TransferInput)
		want   error
	}{
		{"blank amount", func(in *TransferInput) { in.Amount = "" }, ErrTransferAmount},
		{"zero amount", func(in *TransferInput) { in.Amount = "0" }, ErrTransferAmount},
		{"negative amount", func(in *TransferInput) { in.Amount = "-3" }, ErrTransferAmount},
		{"bad fee", func(in *TransferInput) { in.Fee = "nope" }, ErrTransferExtra},
		{"negative received", func(in *TransferInput) { in.Received = "-1" }, ErrTransferExtra},
		{"unparseable date", func(in *TransferInput) { in.Date = "10/31/2026" }, ErrTransferDate},
		{"impossible date", func(in *TransferInput) { in.Date = "2026-02-31" }, ErrTransferDate},
	}
	for _, tc := range errCases {
		t.Run(tc.name, func(t *testing.T) {
			in := base
			tc.mutate(&in)
			if _, err := ParseTransfer(accounts, in); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestTransferFXNote(t *testing.T) {
	accounts := transferAccounts()
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	cases := []struct {
		name         string
		rates        currency.Rates
		from, to     string
		amount       string
		wantKind     FXKind
		wantLandedIn string
	}{
		{"same currency", rates, "chk", "sav", "10", FXNone, ""},
		{"incomplete pick", rates, "chk", "", "10", FXNone, ""},
		{"unknown account", rates, "chk", "ghost", "10", FXNone, ""},
		{"converted at the saved rate", rates, "chk", "travel", "10", FXConverted, "EUR"},
		{"no rate", currency.Rates{Base: "USD"}, "chk", "travel", "10", FXNoRate, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := TransferFXNote(accounts, tc.rates, tc.from, tc.to, tc.amount)
			if got.Kind != tc.wantKind || got.Landed.Currency != tc.wantLandedIn {
				t.Fatalf("note = %+v", got)
			}
		})
	}
	t.Run("landed amount follows the typed amount", func(t *testing.T) {
		typed := TransferFXNote(accounts, rates, "chk", "travel", "10")
		blank := TransferFXNote(accounts, rates, "chk", "travel", "")
		want := func(from int64) int64 {
			conv, err := rates.Convert(money.New(from, "USD"), "EUR")
			if err != nil {
				t.Fatal(err)
			}
			return conv.Amount
		}
		if typed.Landed.Amount != want(1000) {
			t.Fatalf("typed landed = %v", typed.Landed)
		}
		if blank.Landed.Amount != want(100) {
			t.Fatalf("a blank amount previews one whole unit, got %v", blank.Landed)
		}
	})
}

func TestPreviewTransfer(t *testing.T) {
	accounts := transferAccounts()
	t.Run("asset destination", func(t *testing.T) {
		got, ok := PreviewTransfer(accounts, "chk", "sav", "5", "", "0.25")
		if !ok || got.Params.AmountMinor != 500 || got.Params.FeeMinor != 25 || got.FromName != "Checking" || got.ToName != "Savings" || got.ToLiability {
			t.Fatalf("preview = %+v ok %v", got, ok)
		}
	})
	t.Run("liability destination is a payment", func(t *testing.T) {
		got, ok := PreviewTransfer(accounts, "chk", "card", "5", "", "")
		if !ok || !got.ToLiability {
			t.Fatalf("preview = %+v ok %v", got, ok)
		}
	})
	t.Run("invalid extras count as absent", func(t *testing.T) {
		got, ok := PreviewTransfer(accounts, "chk", "travel", "5", "junk", "-1")
		if !ok || got.Params.ReceivedMinor != 0 || got.Params.FeeMinor != 0 {
			t.Fatalf("preview = %+v ok %v", got, ok)
		}
	})
	for name, args := range map[string][5]string{
		"same account":   {"chk", "chk", "5", "", ""},
		"no source":      {"", "sav", "5", "", ""},
		"no destination": {"chk", "", "5", "", ""},
		"blank amount":   {"chk", "sav", "", "", ""},
		"zero amount":    {"chk", "sav", "0", "", ""},
	} {
		t.Run(name, func(t *testing.T) {
			if _, ok := PreviewTransfer(accounts, args[0], args[1], args[2], args[3], args[4]); ok {
				t.Fatal("expected no preview")
			}
		})
	}
}

func TestParseTransferRejectsSelfAndUnknownAccounts(t *testing.T) {
	accounts := transferAccounts()
	cases := []struct {
		name         string
		from, to, am string
		want         error
	}{
		{"same account", "chk", "chk", "5", ErrTransferSameAccount},
		{"unknown source", "ghost", "sav", "5", ErrTransferUnknownAccount},
		{"unknown destination", "chk", "ghost", "5", ErrTransferUnknownAccount},
		{"blank source", "", "sav", "5", ErrTransferUnknownAccount},
		{"blank destination", "chk", "", "5", ErrTransferUnknownAccount},
		{"self transfer outranks a bad amount", "chk", "chk", "nope", ErrTransferSameAccount},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseTransfer(accounts, TransferInput{FromID: tc.from, ToID: tc.to, Amount: tc.am})
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestParseTransferExtraErrorNamesTheBadField(t *testing.T) {
	accounts := transferAccounts()
	cases := []struct {
		name          string
		received, fee string
		wantText      string
	}{
		{"bad received", "x", "", `received amount "x"`},
		{"bad fee", "", "-2", `fee "-2"`},
		{"both bad reports received first", "x", "y", `received amount "x"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseTransfer(accounts, TransferInput{FromID: "chk", ToID: "sav", Amount: "5", Received: tc.received, Fee: tc.fee})
			if !errors.Is(err, ErrTransferExtra) || !strings.Contains(err.Error(), tc.wantText) {
				t.Fatalf("err = %v, want ErrTransferExtra naming %s", err, tc.wantText)
			}
		})
	}
}

func TestTransferCandidateLabelOmitsEmptyCurrencyParentheses(t *testing.T) {
	accounts := []domain.Account{
		{ID: "a", Name: "No currency", Type: domain.TypeChecking},
		{ID: "b", Name: "With currency", Type: domain.TypeChecking, Currency: "USD"},
		{ID: "c", Name: "Already (EUR)", Type: domain.TypeChecking, Currency: "EUR"},
	}
	from, _ := TransferCandidates(accounts, "", "")
	got := map[string]string{}
	for _, c := range from {
		got[c.ID] = c.Label
	}
	want := map[string]string{"a": "No currency", "b": "With currency (USD)", "c": "Already (EUR)"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("labels = %v, want %v", got, want)
	}
}

func TestParseTransferAppliesTheCandidateRules(t *testing.T) {
	accounts := transferAccounts()
	cases := []struct {
		name     string
		from, to string
		want     error
	}{
		{"archived source", "old", "sav", ErrTransferAccountNotAllowed},
		{"archived destination", "chk", "old", ErrTransferAccountNotAllowed},
		{"retirement source is not liquid", "k401", "chk", ErrTransferAccountNotAllowed},
		{"credit card source is not liquid", "card", "chk", ErrTransferAccountNotAllowed},
		{"property destination", "chk", "home", ErrTransferAccountNotAllowed},
		{"vehicle destination", "chk", "car", ErrTransferAccountNotAllowed},
		{"unknown id outranks the rules", "ghost", "home", ErrTransferUnknownAccount},
		{"liquid source to a card is allowed", "chk", "card", nil},
		{"liquid source to a retirement account is allowed", "chk", "k401", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := TransferInput{FromID: tc.from, ToID: tc.to, Amount: "5"}
			_, err := ParseTransfer(accounts, in)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("ParseTransfer err = %v, want nil", err)
				}
				if !CanPostTransfer(accounts, in) {
					t.Fatal("CanPostTransfer = false, want true")
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("ParseTransfer err = %v, want %v", err, tc.want)
			}
			if CanPostTransfer(accounts, in) {
				t.Fatal("CanPostTransfer = true, want false")
			}
		})
	}
}

func TestPreviewTransferRejectsUnknownAndDisallowedAccounts(t *testing.T) {
	accounts := transferAccounts()
	cases := []struct {
		name     string
		from, to string
		want     bool
	}{
		{"unknown source", "ghost", "sav", false},
		{"unknown destination", "chk", "ghost", false},
		{"archived source", "old", "sav", false},
		{"valuation destination", "chk", "home", false},
		{"valid pair", "chk", "sav", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := PreviewTransfer(accounts, tc.from, tc.to, "5", "", ""); ok != tc.want {
				t.Fatalf("PreviewTransfer ok = %v, want %v", ok, tc.want)
			}
		})
	}
}

func TestTransferFXNoteAgreesWithIsCrossCurrency(t *testing.T) {
	accounts := []domain.Account{
		{ID: "usd", Type: domain.TypeChecking, Currency: "USD"},
		{ID: "blank", Type: domain.TypeChecking},
		{ID: "eur", Type: domain.TypeChecking, Currency: "EUR"},
	}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 0.9}}
	for _, tc := range []struct{ from, to string }{{"usd", "blank"}, {"blank", "eur"}, {"usd", "eur"}, {"usd", "usd"}, {"usd", "ghost"}} {
		t.Run(tc.from+"->"+tc.to, func(t *testing.T) {
			note := TransferFXNote(accounts, rates, tc.from, tc.to, "10")
			if got, want := note.Kind != FXNone, IsCrossCurrency(accounts, tc.from, tc.to); got != want {
				t.Fatalf("note shown = %v, IsCrossCurrency = %v", got, want)
			}
		})
	}
}

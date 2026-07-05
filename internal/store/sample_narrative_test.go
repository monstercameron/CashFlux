// SPDX-License-Identifier: MIT

package store

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// TestSampleTrajectory replays the sample's five years of cash flow and asserts
// the invariants that keep the story honest: checking, savings, and cash never
// go negative, and the credit card stays a revolving (negative) balance within
// its limit. This is the guard that a future seed edit can't silently bankrupt
// the Hartleys.
func TestSampleTrajectory(t *testing.T) {
	ds := SampleDataset()
	bal := map[string]int64{}
	limit := map[string]int64{}
	for _, a := range ds.Accounts {
		bal[a.ID] = a.OpeningBalance.Amount
		if a.CreditLimit.Amount != 0 {
			limit[a.ID] = a.CreditLimit.Amount
		}
	}
	txs := append([]domain.Transaction(nil), ds.Transactions...)
	for i := 0; i < len(txs); i++ {
		for j := i + 1; j < len(txs); j++ {
			if txs[j].Date.Before(txs[i].Date) {
				txs[i], txs[j] = txs[j], txs[i]
			}
		}
	}
	mins := map[string]int64{"acct-checking": bal["acct-checking"], "acct-hysa": bal["acct-hysa"], "acct-cash": bal["acct-cash"], "acct-card": bal["acct-card"]}
	for _, tx := range txs {
		if tx.Amount.Currency != "USD" {
			continue // the EUR travel card is out of scope here
		}
		bal[tx.AccountID] += tx.Amount.Amount
		if cur, ok := mins[tx.AccountID]; ok && bal[tx.AccountID] < cur {
			mins[tx.AccountID] = bal[tx.AccountID]
		}
	}
	for _, id := range []string{"acct-checking", "acct-hysa", "acct-cash"} {
		if mins[id] < 0 {
			t.Errorf("%s dips negative: min %.2f", id, float64(mins[id])/100)
		}
	}
	if bal["acct-card"] >= 0 {
		t.Errorf("card should end as a revolving (negative) balance, got %.2f", float64(bal["acct-card"])/100)
	}
	if lim := limit["acct-card"]; lim > 0 && -mins["acct-card"] > lim {
		t.Errorf("card balance exceeds its credit limit: min %.2f vs limit %.2f", float64(mins["acct-card"])/100, float64(lim)/100)
	}
	t.Logf("min checking %.2f, min hysa %.2f, min cash %.2f, final card %.2f",
		float64(mins["acct-checking"])/100, float64(mins["acct-hysa"])/100, float64(mins["acct-cash"])/100, float64(bal["acct-card"])/100)
}

// TestSampleNarrative asserts the five-year story's landmarks so a future seed
// edit can't silently drop an arc: the 2023 layoff gap, the employer switch,
// the 2024 lucky streak, the December gifts, the raw-import errata (an
// uncategorized cluster + a duplicated import), and the spec-backed showcase
// pages.
func TestSampleNarrative(t *testing.T) {
	ds := SampleDataset()
	ym := func(d time.Time) string { return d.Format("2006-01") }

	salaryByMonth := map[string]string{} // month → payee
	unemployment := 0
	streakWins := map[string]int64{}
	uncategorized := 0
	dupKeys := map[string]int{}
	giftMonths := map[string]bool{}
	for _, tx := range ds.Transactions {
		if tx.CategoryID == "cat-salary" && strings.Contains(tx.Desc, "Paycheck") {
			salaryByMonth[ym(tx.Date)] = tx.Payee
		}
		if tx.Payee == "State Workforce Commission" {
			unemployment++
		}
		if tx.CategoryID == "cat-investing-income" && strings.Contains(strings.Join(tx.Tags, ","), "hot-streak") {
			streakWins[ym(tx.Date)] = tx.Amount.Amount
		}
		if tx.CategoryID == "" && tx.TransferAccountID == "" && tx.Source == domain.TxnSourceImported {
			uncategorized++
		}
		if tx.Source == domain.TxnSourceImported {
			dupKeys[tx.Payee+"|"+tx.Date.Format("2006-01-02")+"|"+strconv.FormatInt(tx.Amount.Amount, 10)]++
		}
		if tx.CategoryID == "cat-gifts" && tx.Date.Month() == time.December {
			giftMonths[ym(tx.Date)] = true
		}
	}

	// The layoff gap: no paycheck Feb–May 2023, four unemployment checks, and
	// the employer switches from Cohere to Meridian across the gap.
	for _, month := range []string{"2023-02", "2023-03", "2023-04", "2023-05"} {
		if payee, ok := salaryByMonth[month]; ok {
			t.Errorf("gap month %s should have no paycheck, found one from %q", month, payee)
		}
	}
	if got := salaryByMonth["2023-01"]; got != "Cohere Systems" {
		t.Errorf("Jan 2023 paycheck should be Cohere Systems, got %q", got)
	}
	if got := salaryByMonth["2023-06"]; got != "Meridian Data" {
		t.Errorf("Jun 2023 paycheck should be Meridian Data, got %q", got)
	}
	if unemployment != 4 {
		t.Errorf("want 4 unemployment checks (Feb–May 2023), got %d", unemployment)
	}

	// The lucky streak: four strictly-escalating green months, Feb–May 2024.
	var prev int64
	for _, mth := range []string{"2024-02", "2024-03", "2024-04", "2024-05"} {
		amt, ok := streakWins[mth]
		if !ok {
			t.Errorf("lucky-streak month %s missing its win", mth)
			continue
		}
		if amt <= prev {
			t.Errorf("streak should escalate: %s won %d after %d", mth, amt, prev)
		}
		prev = amt
	}

	// Errata: a healthy uncategorized cluster for the rules/Smart+ demos, and
	// at least one genuinely duplicated import for /duplicates.
	if uncategorized < 40 {
		t.Errorf("want ≥40 uncategorized raw imports (rules/Smart+ fodder), got %d", uncategorized)
	}
	dups := 0
	for _, n := range dupKeys {
		if n > 1 {
			dups++
		}
	}
	if dups == 0 {
		t.Error("want at least one duplicated import (the /duplicates catch)")
	}

	// Every modeled December has holiday gifts.
	for _, yr := range []string{"2021-12", "2022-12", "2023-12", "2024-12", "2025-12"} {
		if !giftMonths[yr] {
			t.Errorf("December %s missing its holiday gifts", yr)
		}
	}

	// The showcase pages' spec-backed widgets validate against the engine's
	// binding rules (exactly one binding per kind).
	specs := 0
	for _, p := range ds.CustomPages {
		for _, w := range p.Widgets {
			if w.Spec == nil {
				continue
			}
			specs++
			if err := w.Spec.Validate(); err != nil {
				t.Errorf("page %s widget %s: invalid spec: %v", p.Slug, w.ID, err)
			}
		}
	}
	if specs < 5 {
		t.Errorf("want ≥5 spec-backed page widgets, got %d", specs)
	}

	// Holdings and snapshots reference real accounts.
	accounts := map[string]bool{}
	for _, a := range ds.Accounts {
		accounts[a.ID] = true
	}
	for _, h := range ds.Holdings {
		if !accounts[h.AccountID] {
			t.Errorf("holding %s: unknown account %q", h.ID, h.AccountID)
		}
	}
	for _, sn := range ds.BalanceSnapshots {
		if !accounts[sn.AccountID] {
			t.Errorf("snapshot %s: unknown account %q", sn.ID, sn.AccountID)
		}
	}
}

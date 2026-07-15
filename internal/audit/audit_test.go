// SPDX-License-Identifier: MIT

package audit

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func priced(key string, page smart.Page, sev smart.Severity, minor int64) smart.Insight {
	return smart.Insight{Feature: "SMART-X", Page: page, Key: key, Title: key, Severity: sev}.
		WithAmount(money.New(minor, "USD"))
}

func TestAuditRanksByImpactThenSeverity(t *testing.T) {
	in := []smart.Insight{
		priced("small", smart.PageBudgets, smart.SeverityAlert, 500),
		priced("big", smart.PageAccounts, smart.SeverityNudge, 34000),
		priced("mid-a", smart.PageGoals, smart.SeverityWarn, 1000),
		priced("mid-b", smart.PageGoals, smart.SeverityAlert, 1000),
	}
	r := Audit(in, "USD")
	gotOrder := []string{}
	for _, f := range r.Findings {
		gotOrder = append(gotOrder, f.Insight.Key)
	}
	want := []string{"big", "mid-b", "mid-a", "small"}
	for i := range want {
		if gotOrder[i] != want[i] {
			t.Fatalf("order[%d] = %q, want %q (full %v)", i, gotOrder[i], want[i], gotOrder)
		}
	}
	if r.TotalImpactMinor != 500+34000+1000+1000 {
		t.Fatalf("total = %d", r.TotalImpactMinor)
	}
	if got := r.TotalImpact(); got.Amount != r.TotalImpactMinor || got.Currency != "USD" {
		t.Fatalf("TotalImpact = %+v", got)
	}
}

func TestAuditImpactUsesAbsoluteAndUnpricedRanksLast(t *testing.T) {
	neg := smart.Insight{Key: "neg", Page: smart.PageAccounts}.WithAmount(money.New(-2000, "USD"))
	info := smart.Insight{Key: "info", Page: smart.PageTransactions} // no amount
	r := Audit([]smart.Insight{info, neg}, "USD")
	if r.Findings[0].Insight.Key != "neg" {
		t.Fatalf("priced (even negative) should rank first, got %q", r.Findings[0].Insight.Key)
	}
	if r.Findings[0].ImpactMinor != 2000 {
		t.Fatalf("abs impact = %d, want 2000", r.Findings[0].ImpactMinor)
	}
	if r.Findings[1].ImpactMinor != 0 {
		t.Fatalf("unpriced impact = %d, want 0", r.Findings[1].ImpactMinor)
	}
}

func TestAuditFixFromAction(t *testing.T) {
	withFix := smart.Insight{Key: "sub", Page: smart.PageSubscriptions}.
		WithAmount(money.New(1200, "USD")).
		WithAction(smart.Action{Kind: smart.ActionCancelSubscription, Label: "Cancel it", SubscriptionName: "Hulu"})
	infoOnly := smart.Insight{Key: "obs", Page: smart.PageBudgets}.WithAmount(money.New(5000, "USD"))
	r := Audit([]smart.Insight{withFix, infoOnly}, "USD")
	if r.OneTapCount() != 1 {
		t.Fatalf("OneTapCount = %d, want 1", r.OneTapCount())
	}
	var subFix Fix
	for _, f := range r.Findings {
		if f.Insight.Key == "sub" {
			subFix = f.Fix
		}
	}
	if !subFix.OneTap || subFix.Kind != smart.ActionCancelSubscription || subFix.Label != "Cancel it" {
		t.Fatalf("sub fix = %+v", subFix)
	}
}

func TestAuditFamilyLabel(t *testing.T) {
	r := Audit([]smart.Insight{
		priced("a", smart.PageAccounts, smart.SeverityInfo, 1),
		{Key: "b"}, // unknown page
	}, "USD")
	byKey := map[string]string{}
	for _, f := range r.Findings {
		byKey[f.Insight.Key] = f.Family
	}
	if byKey["a"] != "Accounts" {
		t.Fatalf("family a = %q", byKey["a"])
	}
	if byKey["b"] != "General" {
		t.Fatalf("family b = %q, want General", byKey["b"])
	}
}

func TestAuditEmpty(t *testing.T) {
	r := Audit(nil, "USD")
	if len(r.Findings) != 0 || r.TotalImpactMinor != 0 || r.OneTapCount() != 0 {
		t.Fatalf("empty audit = %+v", r)
	}
}

// SPDX-License-Identifier: MIT

package appstate

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
)

func TestDeleteCategoryGuardsSplitMultiBudgetRuleRecurringAndWidgetReferences(t *testing.T) {
	a := newApp(t, false)
	for _, category := range []domain.Category{
		{ID: "used", Name: "Used", Kind: domain.KindExpense},
		{ID: "other", Name: "Other", Kind: domain.KindExpense},
	} {
		if err := a.PutCategory(category); err != nil {
			t.Fatal(err)
		}
	}
	account := domain.Account{ID: "cash", Name: "Cash", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}
	if err := a.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	if err := a.PutTransaction(domain.Transaction{
		ID: "split", AccountID: account.ID, Desc: "Market purchase", Date: time.Now(), Amount: money.New(-100, "USD"),
		Splits: []domain.CategorySplit{{CategoryID: "used", Amount: money.New(-100, "USD")}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.store.PutBudget(domain.Budget{
		ID: "multi", Name: "Several", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID,
		CategoryID: "other", CategoryIDs: []string{"other", "used"}, Period: domain.PeriodMonthly,
		Limit: money.New(1000, "USD"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.PutRule(rules.Rule{ID: "rule", Match: "market", SetCategoryID: "used"}); err != nil {
		t.Fatal(err)
	}
	if err := a.PutRecurring(domain.Recurring{ID: "recurring", Label: "Market", Amount: money.New(-100, "USD"), Cadence: domain.CadenceMonthly, CategoryID: "used"}); err != nil {
		t.Fatal(err)
	}
	page := domain.CustomPage{
		ID: "page", Slug: "page", Name: "Page",
		Widgets: []domain.PageWidget{{ID: "flow", Type: "chart", Spec: &domain.WidgetSpec{
			ID: "flow", Kind: domain.KindChart,
			Pipeline: &domain.Pipeline{Source: domain.Source{Kind: domain.SourceSeries, Series: domain.SeriesSpec{Metric: "flow", Filter: "cat:used"}}},
		}}},
	}
	if err := a.PutCustomPage(page); err != nil {
		t.Fatal(err)
	}

	refs, err := a.CategoryDeleteReferences("used")
	if err != nil {
		t.Fatalf("CategoryDeleteReferences: %v", err)
	}
	if refs.Splits != 1 || refs.Budgets != 1 || refs.Rules != 1 || refs.Recurring != 1 || refs.Widgets != 1 || refs.Total() != 5 {
		t.Fatalf("reference counts = %+v, want split/budget/rule/recurring/widget each counted", refs)
	}
	if err := a.DeleteCategory("used"); err == nil || !strings.Contains(err.Error(), "reassign") {
		t.Fatalf("DeleteCategory error = %v, want guarded reassignment error", err)
	}
	if _, ok, err := a.store.GetCategory("used"); err != nil || !ok {
		t.Fatal("guarded deletion removed the still-referenced category")
	}
}

func TestDeleteCategoryStillReparentsChildrenWithoutTreatingThemAsDataRefs(t *testing.T) {
	a := newApp(t, false)
	for _, category := range []domain.Category{
		{ID: "parent", Name: "Parent", Kind: domain.KindExpense},
		{ID: "child", Name: "Child", Kind: domain.KindExpense, ParentID: "parent"},
	} {
		if err := a.PutCategory(category); err != nil {
			t.Fatal(err)
		}
	}
	refs, err := a.CategoryDeleteReferences("parent")
	if err != nil || !refs.Empty() {
		t.Fatalf("child-only references = %+v, err=%v; child handling stays in delete's reparent policy", refs, err)
	}
	if err := a.DeleteCategory("parent"); err != nil {
		t.Fatalf("DeleteCategory with child: %v", err)
	}
	child, ok, err := a.store.GetCategory("child")
	if err != nil || !ok || child.ParentID != "" {
		t.Fatalf("child after parent delete = %+v, exists=%t; want child reparented to root", child, ok)
	}
}

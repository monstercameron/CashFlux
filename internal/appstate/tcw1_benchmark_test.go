package appstate

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// BenchmarkTCW1CSV measures the public CSV seam with aggregate automation and
// an unresolved task. Setup is outside the timer; parsing and staging are inside.
func BenchmarkTCW1CSV(b *testing.B) {
	for _, count := range []int{300, 3000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				b.StopTimer()
				a, err := New(io.Discard, false)
				if err != nil {
					b.Fatal(err)
				}
				account := domain.Account{ID: "tcw1", Name: "Checking", Currency: "USD", Class: domain.ClassAsset, Type: domain.TypeChecking, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}
				if err := a.PutAccount(account); err != nil {
					b.Fatal(err)
				}
				date := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
				a.now = func() time.Time { return date }
				existing := make([]domain.Transaction, 6300)
				for i := range existing {
					existing[i] = domain.Transaction{ID: fmt.Sprint(i), AccountID: account.ID, Date: date, Desc: fmt.Sprintf("History %d", i), Amount: money.New(-100, "USD")}
				}
				if err := a.store.PutTransactionsAtomic(existing); err != nil {
					b.Fatal(err)
				}
				if err := a.PutWorkflow(workflow.Workflow{ID: "tag", Name: "Tag purchases", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded}, Condition: "safe_to_spend < 999999999", Actions: []workflow.Action{{Kind: workflow.ActionAddTag, Tag: "imported"}}}); err != nil {
					b.Fatal(err)
				}
				if err := a.PutTask(domain.Task{ID: "resolve", Title: "Wait", Status: domain.StatusOpen, Priority: domain.PriorityMedium, Resolve: &domain.TaskResolve{Condition: "txn_abs > 999999"}}); err != nil {
					b.Fatal(err)
				}
				var csv strings.Builder
				csv.WriteString("date,account_id,desc,amount\n")
				for i := 0; i < count; i++ {
					fmt.Fprintf(&csv, "2026-10-01,tcw1,New purchase %d,-1.25\n", i)
				}
				data := []byte(csv.String())
				b.StartTimer()
				result, err := a.ImportTransactionsCSV(data, account.ID, "")
				b.StopTimer()
				if err != nil || result.Imported != count {
					b.Fatalf("imported=%d err=%v", result.Imported, err)
				}
				if len(a.WorkflowRuns()) != count {
					b.Fatal("missing workflow runs")
				}
				if err := a.store.Close(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// SPDX-License-Identifier: MIT

package aitools

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/agent"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

type fakeSrc struct {
	txns          []domain.Transaction
	accts         []domain.Account
	liquid, month int64
}

func (f fakeSrc) Transactions() []domain.Transaction { return f.txns }
func (f fakeSrc) Accounts() []domain.Account         { return f.accts }
func (f fakeSrc) Balance(domain.Account) int64       { return 200000 }
func (f fakeSrc) LiquidBalance() int64               { return f.liquid }
func (f fakeSrc) MonthlyNet() int64                  { return f.month }
func (f fakeSrc) ParseMoney(s string) (int64, error) { return money.ParseMinor(s, 2) }
func (f fakeSrc) FormatMoney(m int64) string {
	neg := m < 0
	if neg {
		m = -m
	}
	s := fmt.Sprintf("$%d.%02d", m/100, m%100)
	if neg {
		s = "-" + s
	}
	return s
}

func txn(cat string, minor int64) domain.Transaction {
	return domain.Transaction{CategoryID: cat, Amount: money.New(minor, "USD")}
}

func newReg(src DataSource) *agent.Registry {
	reg := agent.NewRegistry()
	RegisterRead(reg, src)
	return reg
}

func call(t *testing.T, reg *agent.Registry, name, args string) string {
	t.Helper()
	tool, ok := reg.Get(name)
	if !ok {
		t.Fatalf("tool %q not registered", name)
	}
	out, err := tool.Handler(json.RawMessage(args))
	if err != nil {
		t.Fatalf("%s handler error: %v", name, err)
	}
	return out
}

func sampleSource() fakeSrc {
	return fakeSrc{
		txns:   []domain.Transaction{txn("food", -5000), txn("food", -3000), txn("rent", -100000)},
		accts:  []domain.Account{{Name: "Checking"}},
		liquid: 200000, month: 50000,
	}
}

func TestRegisterReadRegistersTools(t *testing.T) {
	reg := newReg(sampleSource())
	names := map[string]bool{}
	for _, s := range reg.Specs() {
		names[s.Name] = true
	}
	for _, want := range []string{"query_transactions", "account_balances", "affordability"} {
		if !names[want] {
			t.Errorf("missing read tool %q", want)
		}
	}
}

func TestQueryTransactions(t *testing.T) {
	reg := newReg(sampleSource())
	if got := call(t, reg, "query_transactions", `{"categories":["food"]}`); got != "2 transactions, net -$80.00." {
		t.Errorf("query = %q, want 2 food txns netting -$80.00", got)
	}
	// No filter → all three.
	if got := call(t, reg, "query_transactions", `{}`); !strings.HasPrefix(got, "3 transactions") {
		t.Errorf("unfiltered query = %q, want all 3", got)
	}
}

func TestAccountBalances(t *testing.T) {
	if got := call(t, newReg(sampleSource()), "account_balances", `{}`); got != "Checking: $2000.00" {
		t.Errorf("balances = %q, want Checking: $2000.00", got)
	}
}

func TestAffordability(t *testing.T) {
	reg := newReg(sampleSource())
	if got := call(t, reg, "affordability", `{"amount":"500.00","months":0}`); !strings.HasPrefix(got, "Yes") {
		t.Errorf("affordable purchase = %q, want a Yes", got)
	}
	got := call(t, reg, "affordability", `{"amount":"99999.00"}`)
	if !strings.Contains(got, "Not yet") || !strings.Contains(got, "months") {
		t.Errorf("unaffordable purchase = %q, want a months-needed estimate", got)
	}
}

func TestQueryTransactionsBadArgs(t *testing.T) {
	reg := newReg(sampleSource())
	tool, _ := reg.Get("query_transactions")
	if _, err := tool.Handler(json.RawMessage(`{not json`)); err == nil {
		t.Error("malformed args should error (the loop turns it into a recoverable tool result)")
	}
}

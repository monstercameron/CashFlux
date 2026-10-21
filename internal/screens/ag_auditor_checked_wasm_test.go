// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/audit"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func TestAssistantAuditFreshReadFailuresAreNotCleanVerdicts(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	tool := assistantFinanceTestTool(t, agToolsAuditor(app, "JPY", currency.Rates{Base: "JPY"}), "run_audit")
	out := tool.run(json.RawMessage(`{}`))
	if !strings.Contains(out, "This does not establish that all financial issues are absent") || strings.Contains(out, "Nothing is bleeding money") {
		t.Fatalf("empty input overclaimed: %s", out)
	}
	if err := app.Store().SetSettingKV("cashflux:smart-settings", "{"); err != nil {
		t.Fatal(err)
	}
	out = tool.run(json.RawMessage(`{}`))
	if !strings.HasPrefix(out, "Audit unavailable: decode audit detector settings:") {
		t.Fatalf("malformed detector settings hidden: %s", out)
	}
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	out = tool.run(json.RawMessage(`{}`))
	if !strings.HasPrefix(out, "Audit unavailable: read audit settings:") || strings.Contains(out, "no findings") {
		t.Fatalf("closed store looked clean: %s", out)
	}
}

func TestAssistantAuditReloadsHouseholdFXAndMissingCurrencyIsUnavailable(t *testing.T) {
	app := agrApp(t)
	t.Cleanup(func() { _ = app.Store().Close() })
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	a := netWorthFixtureAccount("audit-euro", "EUR", 10000, false)
	if err := app.PutAccount(a); err != nil {
		t.Fatal(err)
	}
	settings := app.Settings()
	settings.BaseCurrency = "USD"
	settings.FXRates = nil
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	tool := assistantFinanceTestTool(t, agToolsAuditor(app, "JPY", currency.Rates{Base: "JPY", Rates: map[string]float64{"EUR": 999}}), "run_audit")
	out := tool.run(json.RawMessage(`{}`))
	if !strings.Contains(out, "Audit unavailable: audit input account audit-euro currency:") {
		t.Fatalf("unavailable FX omitted or captured FX used: %s", out)
	}
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	in, _, err := assistantAuditInput(app, now)
	if err != nil || in.Base != "USD" || in.Rates.Rates["EUR"] != 2 || !in.Now.Equal(now) {
		t.Fatalf("fresh input: %+v / %v", in, err)
	}
	settings.FXRates["EUR"] = 3
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	in, _, err = assistantAuditInput(app, now)
	if err != nil || in.Rates.Rates["EUR"] != 3 {
		t.Fatalf("same app retained FX: %+v / %v", in, err)
	}
	if out = tool.run(json.RawMessage(`{}`)); strings.HasPrefix(out, "Audit unavailable:") || strings.Contains(out, "JPY") {
		t.Fatalf("fresh rates did not restore usable input: %s", out)
	}
}

func TestAssistantAuditImpactOutputRetainsPeriodsAndOriginalCurrency(t *testing.T) {
	rep, err := audit.AuditChecked([]smart.Insight{
		smart.Insight{Key: "monthly"}.WithMonthlyAmount(money.New(100, "EUR")),
		smart.Insight{Key: "once"}.WithOneTimeAmount(money.New(500, "USD")),
		smart.Insight{Key: "context"}.WithAmount(money.New(99999, "USD")),
	}, "USD", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"24.00 USD/year", "5.00 USD", "1 unclassified context amounts", "may overlap"} {
		if out := auditSummary(rep); !strings.Contains(out, want) {
			t.Fatalf("missing %q: %s", want, out)
		}
	}
	monthly := auditImpact(rep.Findings[0], rep.Base)
	if !strings.Contains(monthly, "2.00 USD/month") || !strings.Contains(monthly, "1.00 EUR") {
		t.Fatalf("currency/period lost: %s", monthly)
	}
	if !strings.Contains(auditImpact(rep.Findings[1], rep.Base), "one time") || !strings.Contains(auditImpact(rep.Findings[2], rep.Base), "impact period unclassified") {
		t.Fatalf("incompatible period labels lost: %+v", rep.Findings)
	}
	// A paid occurrence comes from the freshly read links, not cached app reads.
	app := agrApp(t)
	t.Cleanup(func() { _ = app.Store().Close() })
	due := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	if err := app.Store().PutTxnLink(domain.TxnLink{ID: "audit-paid", Kind: domain.TxnLinkBillMatch, TxnIDs: []string{"cc33333333333333"}, RecurringID: "audit-rule", OccurrenceDate: due}); err != nil {
		t.Fatal(err)
	}
	in, _, err := assistantAuditInput(app, due)
	if err != nil || !in.OccurrencePaid("audit-rule", due) || in.Ledger == nil {
		t.Fatalf("fresh linked evidence missing: %+v / %v", in, err)
	}
}

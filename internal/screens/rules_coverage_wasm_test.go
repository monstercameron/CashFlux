//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestRuleRowKeepsPreparedCountAndLazyPreview(t *testing.T) {
	previewCalls := 0
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(RuleRow, ruleRowProps{
		Rule:           rules.Rule{ID: "coverage-row", Match: "cafe"},
		Precedence:     1,
		MatchCount:     3,
		MaxMatchCount:  5,
		ShowMatchCount: true,
		PreviewMatches: func(limit int) ([]ruleMatchLine, int) {
			previewCalls++
			return []ruleMatchLine{{Date: "Jun 15, 2026", Desc: "Corner Cafe", Amount: "-$4.50"}}, 3
		},
	}))
	if previewCalls != 0 {
		t.Fatalf("preview ran during initial render: %d calls", previewCalls)
	}
	byTestID(f, "button", "rule-matches-btn-coverage-row").Click()
	f.Stabilize()
	if previewCalls != 1 {
		t.Fatalf("preview calls after opening count = %d, want 1", previewCalls)
	}
	if byTestID(f, "div", "rule-matches-list-coverage-row") == nil {
		t.Fatalf("opening the prepared count did not reveal its lazy preview: %s", f.Text())
	}
}

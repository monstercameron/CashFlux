package aiprovider

import "testing"

func TestProvidersSortedAndComplete(t *testing.T) {
	ps := Providers()
	wantIDs := []string{"anthropic", "cerebras", "deepseek", "glm", "kimi", "openai", "openrouter"}
	if len(ps) != len(wantIDs) {
		t.Fatalf("got %d providers, want %d", len(ps), len(wantIDs))
	}
	for i, id := range wantIDs {
		if ps[i].ID != id {
			t.Errorf("provider[%d] = %q, want %q (sorted by id)", i, ps[i].ID, id)
		}
	}
}

func TestEveryProviderWellFormed(t *testing.T) {
	for _, p := range Providers() {
		if p.Label == "" || p.BaseURL == "" || p.KeyURL == "" {
			t.Errorf("%s: missing label/base/key URL", p.ID)
		}
		if p.Dialect != DialectOpenAI && p.Dialect != DialectAnthropic {
			t.Errorf("%s: invalid dialect %q", p.ID, p.Dialect)
		}
		if len(p.Models) == 0 {
			t.Errorf("%s: has no models", p.ID)
		}
		for _, m := range p.Models {
			if m.ID == "" || m.Label == "" {
				t.Errorf("%s: model with empty id/label: %+v", p.ID, m)
			}
			if m.InputCentsPerMTok < 0 || m.OutputCentsPerMTok < 0 {
				t.Errorf("%s/%s: negative pricing", p.ID, m.ID)
			}
		}
	}
}

func TestDialectAuthPairing(t *testing.T) {
	for _, p := range Providers() {
		switch p.Dialect {
		case DialectAnthropic:
			if p.ID != "anthropic" {
				t.Errorf("%s: only anthropic should use the anthropic dialect", p.ID)
			}
			if p.Auth != AuthXAPIKey {
				t.Errorf("%s: anthropic dialect should use x-api-key auth", p.ID)
			}
		case DialectOpenAI:
			if p.Auth != AuthBearer {
				t.Errorf("%s: openai dialect should use bearer auth", p.ID)
			}
		}
	}
}

func TestFreeTextOnlyOpenRouter(t *testing.T) {
	for _, p := range Providers() {
		if p.FreeText && p.ID != "openrouter" {
			t.Errorf("%s: unexpected free-text provider (only the OpenRouter aggregator should be)", p.ID)
		}
	}
	if or, _ := Get("openrouter"); !or.FreeText {
		t.Error("openrouter should accept free-text model ids")
	}
}

func TestGetAndModelLookup(t *testing.T) {
	if _, ok := Get("nope"); ok {
		t.Error("Get(unknown) should report not found")
	}
	p, ok := Get("openai")
	if !ok {
		t.Fatal("Get(openai) should be found")
	}
	if m, ok := p.Model("gpt-4o"); !ok || !m.Caps.Vision || m.Caps.Structured != StructuredJSONSchema {
		t.Errorf("openai/gpt-4o lookup wrong: %+v ok=%v", m, ok)
	}
	if _, ok := p.Model("ghost"); ok {
		t.Error("unknown model should report not found")
	}
}

func TestDefault(t *testing.T) {
	p, m := Default()
	if p.ID != "openai" || m.ID != "gpt-5.5" {
		t.Errorf("Default = %s/%s, want openai/gpt-5.5", p.ID, m.ID)
	}
	if !m.Caps.Reasoning {
		t.Error("the default model should be a reasoning model")
	}
}

func TestEstimateCents(t *testing.T) {
	gpt4o := Model{InputCentsPerMTok: 250, OutputCentsPerMTok: 1000}
	tests := []struct {
		name    string
		in, out int64
		want    int64
	}{
		{"one million input = 250c", 1_000_000, 0, 250},
		{"one million output = 1000c", 0, 1_000_000, 1000},
		{"mixed", 2_000_000, 500_000, 1000},   // 500 + 500
		{"exact one cent", 4_000, 0, 1},       // 4000*250/1e6 = 1.0 → 1
		{"rounds half up", 6_000, 0, 2},       // 6000*250/1e6 = 1.5 → 2
		{"sub-cent rounds down", 1_000, 0, 0}, // 1000*250/1e6 = 0.25 → 0
		{"negatives clamp to zero", -5, -5, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := EstimateCents(gpt4o, tc.in, tc.out); got != tc.want {
				t.Errorf("EstimateCents(%d,%d) = %d, want %d", tc.in, tc.out, got, tc.want)
			}
		})
	}
}

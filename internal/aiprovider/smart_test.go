// SPDX-License-Identifier: MIT

package aiprovider

import "testing"

func TestSmartModelResolves(t *testing.T) {
	p, m, ok := SmartModel()
	if !ok {
		t.Fatalf("SmartModel not found in registry")
	}
	if p.ID != "openai" {
		t.Errorf("provider = %q, want openai", p.ID)
	}
	if m.ID != SmartModelID {
		t.Errorf("model = %q, want %q", m.ID, SmartModelID)
	}
	if !m.Caps.Reasoning {
		t.Errorf("smart model must be reasoning-capable for effort routing")
	}
	if m.InputCentsPerMTok <= 0 || m.OutputCentsPerMTok <= 0 {
		t.Errorf("smart model must carry indicative pricing, got in=%d out=%d", m.InputCentsPerMTok, m.OutputCentsPerMTok)
	}
}

func TestSmartEscalationModelResolves(t *testing.T) {
	_, m, ok := SmartEscalationModel()
	if !ok {
		t.Fatalf("SmartEscalationModel not found in registry")
	}
	if m.ID != SmartEscalationModelID {
		t.Errorf("model = %q, want %q", m.ID, SmartEscalationModelID)
	}
}

func TestSmartModelCheaperThanEscalation(t *testing.T) {
	_, base, _ := SmartModel()
	_, esc, _ := SmartEscalationModel()
	// The whole point of the default is that it is cheaper to run.
	if base.InputCentsPerMTok >= esc.InputCentsPerMTok || base.OutputCentsPerMTok >= esc.OutputCentsPerMTok {
		t.Errorf("smart default (%d/%d) should be cheaper than escalation (%d/%d)",
			base.InputCentsPerMTok, base.OutputCentsPerMTok, esc.InputCentsPerMTok, esc.OutputCentsPerMTok)
	}
}

func TestSmartProfiles(t *testing.T) {
	if got := SmartProfile().Effort; got != EffortMedium {
		t.Errorf("SmartProfile effort = %q, want medium", got)
	}
	if got := SmartEscalationProfile().Effort; got != EffortLow {
		t.Errorf("SmartEscalationProfile effort = %q, want low (escalate capability, not effort)", got)
	}
}

func TestSmartProfileResolvesForModel(t *testing.T) {
	p, m, _ := SmartModel()
	prof := p.For(m, SmartProfile())
	if prof.API != APIResponses {
		t.Errorf("smart call should use the Responses API, got %q", prof.API)
	}
	if prof.Effort != EffortMedium {
		t.Errorf("reasoning model should keep medium effort, got %q", prof.Effort)
	}
}

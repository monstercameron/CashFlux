// SPDX-License-Identifier: MIT

package aiprovider

import "testing"

func TestDefaultProfile(t *testing.T) {
	p := DefaultProfile()
	if p.API != APIResponses || p.Transport != TransportWebSocket || !p.Stream || p.Effort != EffortMedium {
		t.Errorf("DefaultProfile = %+v, want Responses/websocket/stream/medium", p)
	}
	if eff := LowEffortProfile().Effort; eff != EffortLow {
		t.Errorf("LowEffortProfile effort = %q, want low", eff)
	}
}

func TestProfileFor(t *testing.T) {
	openai, _ := Get("openai")
	gpt55, _ := openai.Model("gpt-5.5")
	// The default OpenAI reasoning model keeps the full preferred shape.
	if got := openai.For(gpt55, DefaultProfile()); got != DefaultProfile() {
		t.Errorf("openai/gpt-5.5 profile = %+v, want the unchanged default", got)
	}

	// A non-reasoning OpenAI model drops the effort but keeps Responses/websocket.
	mini, _ := openai.Model("gpt-4o-mini")
	gm := openai.For(mini, DefaultProfile())
	if gm.Effort != "" || gm.API != APIResponses {
		t.Errorf("non-reasoning model profile = %+v, want no effort but Responses kept", gm)
	}

	// A non-OpenAI dialect (Anthropic) falls back to chat-completions over HTTPS.
	anth, _ := Get("anthropic")
	am, _ := anth.Model("claude-3-5-sonnet-latest")
	ap := anth.For(am, DefaultProfile())
	if ap.API != APIChatCompletions || ap.Transport != TransportHTTPS {
		t.Errorf("anthropic profile = %+v, want chat_completions over https", ap)
	}
}

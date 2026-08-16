// SPDX-License-Identifier: MIT

package aiprovider

import "testing"

func TestTypicalQuestionCostsWhatThePricingSays(t *testing.T) {
	// 100 cents per million in, 200 out → 9000 in + 500 out.
	m := Model{ID: "x", InputCentsPerMTok: 100, OutputCentsPerMTok: 200}
	got, ok := m.TypicalQuestionUSD()
	if !ok {
		t.Fatal("a priced model reported no cost")
	}
	want := (100*9000/1e6 + 200*500/1e6) / 100
	if got != want {
		t.Fatalf("cost = %v, want %v", got, want)
	}
}

func TestAnUnpricedModelSaysNothingRatherThanGuessing(t *testing.T) {
	if _, ok := (Model{ID: "mystery"}).TypicalQuestionUSD(); ok {
		t.Fatal("an unpriced model produced a cost — an invented price is worse than an absent one")
	}
}

func TestProviderQuotesItsCheapestModel(t *testing.T) {
	p := Provider{Models: []Model{
		{ID: "big", InputCentsPerMTok: 1000, OutputCentsPerMTok: 2000},
		{ID: "small", InputCentsPerMTok: 10, OutputCentsPerMTok: 20},
		{ID: "unpriced"},
	}}
	got, ok := p.CheapestTypicalQuestionUSD()
	if !ok {
		t.Fatal("no cost from a provider with priced models")
	}
	small, _ := Model{InputCentsPerMTok: 10, OutputCentsPerMTok: 20}.TypicalQuestionUSD()
	if got != small {
		t.Fatalf("quoted %v, want the cheapest model's %v", got, small)
	}
}

func TestAProviderWithNoPricingQuotesNothing(t *testing.T) {
	if _, ok := (Provider{Models: []Model{{ID: "a"}}}).CheapestTypicalQuestionUSD(); ok {
		t.Fatal("an unpriced provider produced a cost")
	}
}

func TestTheCuratedOpenAIProviderCanQuoteAPrice(t *testing.T) {
	// The key gate quotes this. If the registry ever loses its pricing the gate
	// silently stops answering "what will this cost me?", which is the question
	// that stops people adding a key.
	p, ok := Get("openai")
	if !ok {
		t.Fatal("no openai provider in the registry")
	}
	cost, ok := p.CheapestTypicalQuestionUSD()
	if !ok {
		t.Fatal("the openai provider carries no pricing, so the key gate cannot quote one")
	}
	if cost <= 0 || cost > 1 {
		t.Fatalf("a typical question costs $%v, which is not a believable figure", cost)
	}
}

func TestForBaseURLIdentifiesTheProvider(t *testing.T) {
	p, ok := ForBaseURL("https://api.openai.com/v1")
	if !ok || p.ID != "openai" {
		t.Fatalf("provider = %q/%v", p.ID, ok)
	}
	// Trailing slashes and case are how people actually paste a URL.
	if p, ok := ForBaseURL("HTTPS://API.OPENAI.COM/V1/"); !ok || p.ID != "openai" {
		t.Fatalf("a differently-cased URL was not recognised: %q/%v", p.ID, ok)
	}
	if p, ok := ForBaseURL("https://openrouter.ai/api/v1"); !ok || p.ID != "openrouter" {
		t.Fatalf("provider = %q/%v", p.ID, ok)
	}
}

func TestAnEmptyBaseURLMeansTheDefault(t *testing.T) {
	p, ok := ForBaseURL("")
	if !ok || p.ID != "openai" {
		t.Fatalf("an unset endpoint should be the default provider, got %q/%v", p.ID, ok)
	}
}

func TestAnUnknownEndpointIsReportedAsUnknown(t *testing.T) {
	// A local model or a proxy must NOT be quoted OpenAI's prices; ok=false is how
	// the caller knows to say less rather than say something wrong.
	if _, ok := ForBaseURL("http://localhost:11434/v1"); ok {
		t.Fatal("a local endpoint was reported as a known provider")
	}
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package ai

// RealProvider wraps the production browser-fetch transport so it satisfies the
// Provider interface. Boot code constructs one of these when the mock flag is
// off; the mock path constructs a MockProvider instead.
type RealProvider struct{}

func (RealProvider) Chat(apiKey, baseURL, model string, messages []Message, temperature float64, onResult func(string, Usage), onError func(string)) func() {
	return SendChat(apiKey, baseURL, model, messages, temperature, onResult, onError)
}

func (RealProvider) VisionChat(apiKey, baseURL, model, systemPrompt, userText, imageURL string, temperature float64, onResult func(string, Usage), onError func(string)) func() {
	return SendVisionChat(apiKey, baseURL, model, systemPrompt, userText, imageURL, temperature, onResult, onError)
}

func (RealProvider) StructuredVisionChat(apiKey, baseURL, model, systemPrompt, userText, imageURL string, temperature float64, schemaName string, schema []byte, onResult func(string, Usage), onError func(string)) func() {
	return SendStructuredVisionChat(apiKey, baseURL, model, systemPrompt, userText, imageURL, temperature, schemaName, schema, onResult, onError)
}

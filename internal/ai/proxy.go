// SPDX-License-Identifier: MIT

package ai

import (
	"encoding/json"
	"strings"
)

type ProxyVisionRequest struct {
	Model        string          `json:"model"`
	SystemPrompt string          `json:"systemPrompt"`
	UserText     string          `json:"userText"`
	ImageURL     string          `json:"imageUrl"`
	Temperature  float64         `json:"temperature,omitempty"`
	SchemaName   string          `json:"schemaName,omitempty"`
	Schema       json.RawMessage `json:"schema,omitempty"`
}

type ProxyCompletion struct {
	Content string `json:"content"`
	Usage   Usage  `json:"usage"`
}

func BuildProxyChatRequest(model string, messages []Message, temperature float64) ([]byte, error) {
	return BuildRequest(model, messages, temperature)
}

func BuildProxyStructuredVisionRequest(model, systemPrompt, userText, imageURL string, temperature float64, schemaName string, schema []byte) ([]byte, error) {
	return json.Marshal(ProxyVisionRequest{
		Model:        strings.TrimSpace(model),
		SystemPrompt: systemPrompt,
		UserText:     userText,
		ImageURL:     imageURL,
		Temperature:  temperature,
		SchemaName:   strings.TrimSpace(schemaName),
		Schema:       json.RawMessage(schema),
	})
}

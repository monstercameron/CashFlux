// SPDX-License-Identifier: MIT

package ai

import "encoding/json"

// visionImageURL is the image reference in a multimodal message part. The URL is
// usually a data: URL (base64-encoded image) so the bytes never touch a server
// other than OpenAI.
type visionImageURL struct {
	URL string `json:"url"`
}

// visionContentPart is one part of a multimodal user message: either a text part
// or an image part.
type visionContentPart struct {
	Type     string          `json:"type"` // "text" or "image_url"
	Text     string          `json:"text,omitempty"`
	ImageURL *visionImageURL `json:"image_url,omitempty"`
}

// visionMessage mirrors Message but allows the content to be either a plain
// string (system/assistant) or an array of parts (a multimodal user message).
type visionMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// visionRequest is a chat-completions request whose user message carries an image.
type visionRequest struct {
	Model          string          `json:"model"`
	Messages       []visionMessage `json:"messages"`
	Temperature    float64         `json:"temperature,omitempty"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}

// visionMessages builds the system + multimodal-user message pair (text + one
// image) shared by the plain and structured vision requests.
func visionMessages(systemPrompt, userText, imageURL string) []visionMessage {
	return []visionMessage{
		{Role: RoleSystem, Content: systemPrompt},
		{Role: RoleUser, Content: []visionContentPart{
			{Type: "text", Text: userText},
			{Type: "image_url", ImageURL: &visionImageURL{URL: imageURL}},
		}},
	}
}

// BuildVisionRequest marshals a multimodal chat request: a system prompt, a user
// text instruction, and one image (as a data: or http: URL). The model's reply is
// plain text and is read with ParseResponse, exactly like a text chat. Use a
// vision-capable model (e.g. gpt-5.5).
func BuildVisionRequest(model, systemPrompt, userText, imageURL string, temperature float64) ([]byte, error) {
	return json.Marshal(visionRequest{Model: model, Messages: visionMessages(systemPrompt, userText, imageURL), Temperature: temperature})
}

// BuildStructuredVisionRequest is BuildVisionRequest plus a JSON-schema
// response_format, so the vision model returns JSON matching schema (decodable
// directly) instead of free-form text. schemaName is a short identifier; schema
// is the raw JSON Schema.
func BuildStructuredVisionRequest(model, systemPrompt, userText, imageURL string, temperature float64, schemaName string, schema []byte) ([]byte, error) {
	return json.Marshal(visionRequest{
		Model:       model,
		Messages:    visionMessages(systemPrompt, userText, imageURL),
		Temperature: temperature,
		ResponseFormat: &ResponseFormat{
			Type:       "json_schema",
			JSONSchema: JSONSchema{Name: schemaName, Schema: json.RawMessage(schema), Strict: true},
		},
	})
}

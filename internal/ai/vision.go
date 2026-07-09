// SPDX-License-Identifier: MIT

package ai

import "encoding/json"

// visionImageURL is the image reference in a multimodal message part. The URL is
// usually a data: URL (base64-encoded image) so the bytes never touch a server
// other than OpenAI.
type visionImageURL struct {
	URL string `json:"url"`
}

// visionFile is a document (e.g. a PDF) attached to a multimodal message. FileData
// is a data: URL ("data:application/pdf;base64,…"). OpenAI extracts BOTH the text
// and the page images from a PDF, so this handles scanned statements too — the bytes
// go only to OpenAI.
type visionFile struct {
	Filename string `json:"filename"`
	FileData string `json:"file_data"`
}

// visionContentPart is one part of a multimodal user message: a text part, an image
// part, or a file part.
type visionContentPart struct {
	Type     string          `json:"type"` // "text" | "image_url" | "file"
	Text     string          `json:"text,omitempty"`
	ImageURL *visionImageURL `json:"image_url,omitempty"`
	File     *visionFile     `json:"file,omitempty"`
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

// fileMessages builds the system + multimodal-user message pair carrying one attached
// document (e.g. a PDF) plus the instruction text. fileData is a data: URL.
func fileMessages(systemPrompt, userText, filename, fileData string) []visionMessage {
	return []visionMessage{
		{Role: RoleSystem, Content: systemPrompt},
		{Role: RoleUser, Content: []visionContentPart{
			{Type: "file", File: &visionFile{Filename: filename, FileData: fileData}},
			{Type: "text", Text: userText},
		}},
	}
}

// BuildStructuredFileRequest marshals a structured chat request that attaches a
// document (PDF) to the user message, with a JSON-schema response_format. The model
// reads the PDF's text and page images natively — no client-side rendering. Use a
// vision-capable model (gpt-4o and later, e.g. gpt-5.5). fileData is a data: URL
// ("data:application/pdf;base64,…").
func BuildStructuredFileRequest(model, systemPrompt, userText, filename, fileData string, temperature float64, schemaName string, schema []byte) ([]byte, error) {
	return json.Marshal(visionRequest{
		Model:       model,
		Messages:    fileMessages(systemPrompt, userText, filename, fileData),
		Temperature: temperature,
		ResponseFormat: &ResponseFormat{
			Type:       "json_schema",
			JSONSchema: JSONSchema{Name: schemaName, Schema: json.RawMessage(schema), Strict: true},
		},
	})
}

// SPDX-License-Identifier: MIT

package i18n

// English strings for the complete, searchable AI model picker.
func init() {
	keys := map[string]string{
		"settings.aiModelSearch":       "Search all model IDs",
		"settings.aiModelUseCustom":    "Use custom model ID: %s",
		"settings.aiModelNotLatest":    "not in the latest list",
		"settings.aiModelKindsHint":    "Every model your account lists is shown. Some models are for images, audio, or embeddings and may not work for chat.",
		"settings.modelGroupChat":      "Chat",
		"settings.modelGroupReasoning": "Reasoning",
		"settings.modelGroupOther":     "Other — may not work for chat",
	}
	for key, value := range keys {
		english[key] = value
	}
}

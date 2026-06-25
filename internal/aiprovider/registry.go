// SPDX-License-Identifier: MIT

package aiprovider

// registry is the curated set of supported providers. Six speak the OpenAI dialect
// (so they share the existing transport — only base URL, key, and model differ);
// Anthropic speaks its own. Model capabilities and the cents-per-million-token
// prices are INDICATIVE seeds for the picker/estimate UI and drift upstream — they
// must be verified at build, not trusted as a contract.
var registry = []Provider{
	{
		ID: "openai", Label: "OpenAI", Dialect: DialectOpenAI, Auth: AuthBearer,
		BaseURL: "https://api.openai.com/v1", KeyURL: "https://platform.openai.com/api-keys",
		Models: []Model{
			{ID: "gpt-5.5", Label: "GPT-5.5", InputCentsPerMTok: 200, OutputCentsPerMTok: 800,
				Caps: Capabilities{Vision: true, Streaming: true, ToolUse: true, Reasoning: true, Structured: StructuredJSONSchema}},
			{ID: "gpt-5.4-mini", Label: "GPT-5.4 mini", InputCentsPerMTok: 25, OutputCentsPerMTok: 200,
				Caps: Capabilities{Vision: true, Streaming: true, ToolUse: true, Reasoning: true, Structured: StructuredJSONSchema}},
		},
	},
	{
		ID: "openrouter", Label: "OpenRouter", Dialect: DialectOpenAI, Auth: AuthBearer, FreeText: true,
		BaseURL: "https://openrouter.ai/api/v1", KeyURL: "https://openrouter.ai/keys",
		Models: []Model{
			{ID: "openai/gpt-5.4-mini", Label: "GPT-5.4 mini (via OpenRouter)", InputCentsPerMTok: 25, OutputCentsPerMTok: 200,
				Caps: Capabilities{Vision: true, Streaming: true, ToolUse: true, Structured: StructuredJSONObject}},
			{ID: "anthropic/claude-3.5-sonnet", Label: "Claude 3.5 Sonnet (via OpenRouter)", InputCentsPerMTok: 300, OutputCentsPerMTok: 1500,
				Caps: Capabilities{Vision: true, Streaming: true, ToolUse: true, Structured: StructuredJSONObject}},
		},
	},
	{
		ID: "cerebras", Label: "Cerebras", Dialect: DialectOpenAI, Auth: AuthBearer,
		BaseURL: "https://api.cerebras.ai/v1", KeyURL: "https://cloud.cerebras.ai",
		Models: []Model{
			{ID: "llama3.1-8b", Label: "Llama 3.1 8B", InputCentsPerMTok: 10, OutputCentsPerMTok: 10,
				Caps: Capabilities{Streaming: true, Structured: StructuredJSONObject}},
			{ID: "llama3.1-70b", Label: "Llama 3.1 70B", InputCentsPerMTok: 60, OutputCentsPerMTok: 60,
				Caps: Capabilities{Streaming: true, Structured: StructuredJSONObject}},
		},
	},
	{
		ID: "deepseek", Label: "DeepSeek", Dialect: DialectOpenAI, Auth: AuthBearer,
		BaseURL: "https://api.deepseek.com/v1", KeyURL: "https://platform.deepseek.com",
		Models: []Model{
			{ID: "deepseek-chat", Label: "DeepSeek Chat", InputCentsPerMTok: 14, OutputCentsPerMTok: 28,
				Caps: Capabilities{Streaming: true, ToolUse: true, Structured: StructuredJSONObject}},
		},
	},
	{
		ID: "glm", Label: "GLM (Zhipu)", Dialect: DialectOpenAI, Auth: AuthBearer,
		BaseURL: "https://open.bigmodel.cn/api/paas/v4", KeyURL: "https://open.bigmodel.cn",
		Models: []Model{
			{ID: "glm-4", Label: "GLM-4", InputCentsPerMTok: 100, OutputCentsPerMTok: 100,
				Caps: Capabilities{Streaming: true, ToolUse: true, Structured: StructuredJSONObject}},
		},
	},
	{
		ID: "kimi", Label: "Kimi (Moonshot)", Dialect: DialectOpenAI, Auth: AuthBearer,
		BaseURL: "https://api.moonshot.cn/v1", KeyURL: "https://platform.moonshot.cn",
		Models: []Model{
			{ID: "moonshot-v1-8k", Label: "Moonshot v1 8K", InputCentsPerMTok: 120, OutputCentsPerMTok: 120,
				Caps: Capabilities{Streaming: true, Structured: StructuredJSONObject}},
		},
	},
	{
		ID: "anthropic", Label: "Anthropic (Claude)", Dialect: DialectAnthropic, Auth: AuthXAPIKey,
		BaseURL: "https://api.anthropic.com/v1", KeyURL: "https://console.anthropic.com/settings/keys",
		Models: []Model{
			{ID: "claude-3-5-sonnet-latest", Label: "Claude 3.5 Sonnet", InputCentsPerMTok: 300, OutputCentsPerMTok: 1500,
				Caps: Capabilities{Vision: true, Streaming: true, ToolUse: true, Structured: StructuredNone}},
			{ID: "claude-3-5-haiku-latest", Label: "Claude 3.5 Haiku", InputCentsPerMTok: 80, OutputCentsPerMTok: 400,
				Caps: Capabilities{Vision: true, Streaming: true, ToolUse: true, Structured: StructuredNone}},
		},
	},
}

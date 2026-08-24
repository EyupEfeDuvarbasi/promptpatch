package llm

type ProviderInfo struct {
	Provider       Provider
	EnvKey         string
	DefaultModel   string
	DefaultURL     string
	RequiresAPIKey bool
	Configurable   bool
}

var providerRegistry = []ProviderInfo{
	{Provider: OpenAI, EnvKey: "OPENAI_API_KEY", DefaultModel: defaultOpenAIModel, DefaultURL: "https://api.openai.com/v1/responses", RequiresAPIKey: true, Configurable: true},
	{Provider: Gemini, EnvKey: "GEMINI_API_KEY", DefaultModel: defaultGeminiModel, DefaultURL: "https://generativelanguage.googleapis.com/v1beta/interactions", RequiresAPIKey: true, Configurable: true},
	{Provider: Anthropic, EnvKey: "ANTHROPIC_API_KEY", DefaultModel: defaultAnthropicModel, DefaultURL: "https://api.anthropic.com/v1/messages", RequiresAPIKey: true, Configurable: true},
	{Provider: Ollama, DefaultModel: defaultOllamaModel, DefaultURL: "http://127.0.0.1:11434/api/generate", RequiresAPIKey: false, Configurable: false},
}

func ProviderRegistry() []ProviderInfo {
	return append([]ProviderInfo(nil), providerRegistry...)
}

func ProviderDetails(provider Provider) (ProviderInfo, bool) {
	for _, info := range providerRegistry {
		if info.Provider == provider {
			return info, true
		}
	}
	return ProviderInfo{}, false
}

func ConfigurableProviders() []ProviderInfo {
	providers := []ProviderInfo{}
	for _, info := range providerRegistry {
		if info.Configurable {
			providers = append(providers, info)
		}
	}
	return providers
}

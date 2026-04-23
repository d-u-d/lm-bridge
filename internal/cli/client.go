package cli

import (
	"lm-bridge/internal/db"
	"lm-bridge/internal/llm"
)

// newClient builds an LLM client from stored provider settings.
// Falls back to default LM Studio if store is nil or no provider is configured.
func newClient(store *db.Store) *llm.Client {
	if store == nil {
		return llm.New("", "", "")
	}
	provider, _ := store.GetSetting("provider")
	switch provider {
	case "openrouter":
		apiKey, _ := store.GetSetting("openrouter_api_key")
		model, _ := store.GetSetting("openrouter_model")
		return llm.New("", apiKey, model)
	default: // "lmstudio" or empty
		baseURL, _ := store.GetSetting("lmstudio_url")
		return llm.New(baseURL, "", "")
	}
}

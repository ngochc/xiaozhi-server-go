package ollama

import (
	"xiaozhi-server-go/src/core/providers/vlllm"
	"xiaozhi-server-go/src/core/utils"
)

// OllamaVLLMProvider Ollama VLLLM provider
type OllamaVLLMProvider struct {
	*vlllm.Provider
}

// NewProvider creates Ollama VLLLM provider instance
func NewProvider(config *vlllm.Config, logger *utils.Logger) (*vlllm.Provider, error) {
	// Directly use BaseVLLMProvider as it reuses LLM architecture
	// Ollama VLLLM requires ensuring correct model name
	provider, err := vlllm.NewProvider(config, logger)
	if err != nil {
		return nil, err
	}

	logger.Debug("Ollama VLLLM Provider created successfully %v", map[string]interface{}{
		"model_name": config.ModelName,
		"base_url":   config.BaseURL,
	})

	return provider, nil
}

// init registers Ollama VLLLM provider
func init() {
	vlllm.Register("ollama", NewProvider)
}

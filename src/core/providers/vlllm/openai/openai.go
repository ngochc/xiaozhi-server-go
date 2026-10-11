package openai

import (
	"xiaozhi-server-go/src/core/providers/vlllm"
	"xiaozhi-server-go/src/core/utils"
)

// OpenAIVLLMProvider OpenAI VLLLM provider
type OpenAIVLLMProvider struct {
	*vlllm.Provider
}

// NewProvider creates OpenAI VLLLM provider instance
func NewProvider(config *vlllm.Config, logger *utils.Logger) (*vlllm.Provider, error) {
	// Directly use BaseVLLMProvider as it reuses LLM architecture
	// OpenAI VLLLM requires ensuring correct model name
	provider, err := vlllm.NewProvider(config, logger)
	if err != nil {
		return nil, err
	}

	logger.Debug("OpenAI VLLLM Provider created successfully %v", map[string]interface{}{
		"model_name": config.ModelName,
		"base_url":   config.BaseURL,
	})

	return provider, nil
}

// init registers OpenAI VLLLM provider
func init() {
	vlllm.Register("openai", NewProvider)
}

package provider

import (
	"fmt"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/mcp"
	"xiaozhi-server-go/src/core/providers"
	"xiaozhi-server-go/src/core/providers/vlllm"
	"xiaozhi-server-go/src/core/utils"
)

// ProviderManager creates providers for each connection and destroys them
// when that connection closes. No provider instances are shared.
type ProviderManager struct {
	config       *configs.Config
	logger       *utils.Logger
	asrFactory   *ProviderFactory
	llmFactory   *ProviderFactory
	ttsFactory   *ProviderFactory
	vlllmFactory *ProviderFactory
	mcpFactory   *ProviderFactory
}

type ProviderSet struct {
	ASR   providers.ASRProvider
	LLM   providers.LLMProvider
	TTS   providers.TTSProvider
	VLLLM *vlllm.Provider
	MCP   *mcp.Manager
}

func NewProviderManager(config *configs.Config, logger *utils.Logger) (*ProviderManager, error) {
	pm := &ProviderManager{
		config: config,
		logger: logger,
	}

	selectedModule := config.SelectedModule
	if selectedModule == nil {
		selectedModule = map[string]string{}
	}

	if asrType := selectedModule["ASR"]; asrType != "" {
		asrFactory := NewASRFactory(asrType, config, logger)
		if asrFactory == nil {
			return nil, fmt.Errorf("failed to create ASR factory: config not found for %s", asrType)
		}
		pm.asrFactory = asrFactory
		logger.Info("[ASR] Selected ASR model: %s", asrType)
	} else {
		logger.Warn("[ASR] Selected ASR model is empty")
	}

	if llmType := selectedModule["LLM"]; llmType != "" {
		llmFactory := NewLLMFactory(llmType, config, logger)
		if llmFactory == nil {
			return nil, fmt.Errorf("failed to create LLM factory: config not found for %s", llmType)
		}
		pm.llmFactory = llmFactory
	} else {
		logger.Warn("[LLM] Selected LLM model is empty")
	}

	if ttsType := selectedModule["TTS"]; ttsType != "" {
		ttsFactory := NewTTSFactory(ttsType, config, logger)
		if ttsFactory == nil {
			return nil, fmt.Errorf("failed to create TTS factory: config not found for %s", ttsType)
		}
		pm.ttsFactory = ttsFactory
		logger.Info("[TTS] Selected TTS model: %s", ttsType)
	} else {
		logger.Warn("[TTS] Selected TTS model is empty")
	}

	if vlllmType := selectedModule["VLLLM"]; vlllmType != "" {
		vlllmFactory := NewVLLLMFactory(vlllmType, config, logger)
		if vlllmFactory == nil {
			logger.Warn("failed to create VLLLM factory: config not found for %s", vlllmType)
		} else {
			pm.vlllmFactory = vlllmFactory
			logger.Info("[VLLLM] Selected VLLLM model: %s", vlllmType)
		}
	}

	pm.mcpFactory = NewMCPFactory(config, logger)
	return pm, nil
}

func (pm *ProviderManager) GetProviderSet() (*ProviderSet, error) {
	set := &ProviderSet{}

	if pm.asrFactory != nil {
		asr, err := pm.asrFactory.Create()
		if err != nil {
			pm.DestroyProviderSet(set)
			return nil, fmt.Errorf("failed to create ASR provider: %v", err)
		}
		set.ASR = asr.(providers.ASRProvider)
	}

	if pm.llmFactory != nil {
		llm, err := pm.llmFactory.Create()
		if err != nil {
			pm.DestroyProviderSet(set)
			return nil, fmt.Errorf("failed to create LLM provider: %v", err)
		}
		set.LLM = llm.(providers.LLMProvider)
	}

	if pm.ttsFactory != nil {
		tts, err := pm.ttsFactory.Create()
		if err != nil {
			pm.DestroyProviderSet(set)
			return nil, fmt.Errorf("failed to create TTS provider: %v", err)
		}
		set.TTS = tts.(providers.TTSProvider)
	}

	if pm.vlllmFactory != nil {
		vlllmProvider, err := pm.vlllmFactory.Create()
		if err != nil {
			pm.logger.Warn("failed to create VLLLM provider, falling back to standard LLM: %v", err)
		} else {
			set.VLLLM = vlllmProvider.(*vlllm.Provider)
		}
	}

	if pm.mcpFactory != nil {
		mcpManager, err := pm.mcpFactory.Create()
		if err != nil {
			pm.DestroyProviderSet(set)
			return nil, fmt.Errorf("failed to create MCP manager: %v", err)
		}
		set.MCP = mcpManager.(*mcp.Manager)
	}

	return set, nil
}

func (pm *ProviderManager) ReturnProviderSet(set *ProviderSet) error {
	return pm.DestroyProviderSet(set)
}

func (pm *ProviderManager) DestroyProviderSet(set *ProviderSet) error {
	if set == nil {
		return nil
	}

	var errs []error
	cleanup := func(name string, resource interface{}) {
		if resource == nil {
			return
		}
		if cleaner, ok := resource.(interface{ Cleanup() error }); ok {
			if err := cleaner.Cleanup(); err != nil {
				errs = append(errs, fmt.Errorf("%s cleanup failed: %v", name, err))
			}
		}
	}

	cleanup("ASR", set.ASR)
	cleanup("LLM", set.LLM)
	cleanup("TTS", set.TTS)
	cleanup("VLLLM", set.VLLLM)
	cleanup("MCP", set.MCP)

	if len(errs) > 0 {
		return fmt.Errorf("error occurred while destroying providers: %v", errs)
	}
	return nil
}

func (pm *ProviderManager) Close() {}

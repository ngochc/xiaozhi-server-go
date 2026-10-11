package webapi

import (
	"encoding/json"
	"fmt"
	"strings"

	"xiaozhi-server-go/src/configs/database"
	"xiaozhi-server-go/src/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type onboardingStatus struct {
	Exists         bool     `json:"exists"`
	Ready          bool     `json:"ready"`
	Issues         []string `json:"issues"`
	AgentID        uint     `json:"agentId,omitempty"`
	LLM            string   `json:"llm,omitempty"`
	Voice          string   `json:"voice,omitempty"`
	PendingDevices int64    `json:"pendingDevices"`
}

// onboardingProviderSetup reads active modules from persisted admin provider configuration.
func (s *DefaultAdminService) onboardingProviderSetup() (string, string, []string) {
	if s.config == nil || s.config.SelectedModule == nil {
		return "", "", []string{"System configuration not yet initialized"}
	}
	issues := make([]string, 0)
	selectedConfigs := make(map[string]map[string]interface{}, 3)
	for _, providerType := range []string{"ASR", "LLM", "TTS"} {
		name := strings.TrimSpace(s.config.SelectedModule[providerType])
		if name == "" {
			issues = append(issues, providerType+": No provider selected")
			continue
		}
		providers, err := database.GetProviderByTypeInternal(providerType, database.AdminUserID, false)
		if err != nil {
			issues = append(issues, providerType+": Failed to read provider config")
			continue
		}
		raw, ok := providers[name]
		if !ok {
			issues = append(issues, fmt.Sprintf("%s: Selected provider %q not found", providerType, name))
			continue
		}
		var providerConfig map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &providerConfig); err != nil {
			issues = append(issues, fmt.Sprintf("%s: Provider config is not valid JSON", providerType))
			continue
		}
		subType := strings.ToLower(strings.TrimSpace(stringConfig(providerConfig, "type")))
		if subType == "" {
			issues = append(issues, fmt.Sprintf("%s: Provider missing specific subtype", providerType))
			continue
		}
		if missing := missingProviderTestFields(providerType, subType, providerConfig); len(missing) > 0 {
			issues = append(issues, fmt.Sprintf("%s %q still requires: %s", providerType, name, strings.Join(missing, ", ")))
			continue
		}
		invalidField := ""
		for _, field := range requiredProviderFields(providerType, subType) {
			if isPlaceholder(stringConfig(providerConfig, field)) {
				invalidField = field
				break
			}
		}
		if invalidField != "" {
			issues = append(issues, fmt.Sprintf("%s '%s' field '%s' is still a placeholder value", providerType, name, invalidField))
			continue
		}
		selectedConfigs[providerType] = providerConfig
	}
	if len(issues) > 0 {
		return "", "", issues
	}
	voice := stringConfig(selectedConfigs["TTS"], "voice")
	if isPlaceholder(voice) {
		return "", "", []string{"TTS: No valid voice configured"}
	}
	return s.config.SelectedModule["LLM"], voice, nil
}

func requiredProviderFields(providerType, subType string) []string {
	required := map[string]map[string][]string{
		"ASR": {
			"doubao": {"appid", "access_token"}, "deepgram": {"addr", "api_key"}, "gosherpa": {"addr"},
			"stepfun": {"api_key", "model", "voice"}, "iflytek": {"appid", "api_key", "api_secret"},
		},
		"TTS": {
			"edge": {"voice"}, "doubao": {"appid", "token", "cluster", "voice"}, "gosherpa": {"cluster"},
			"deepgram": {"cluster", "token", "voice"}, "iflytek": {"appid", "token", "cluster"},
		},
		"LLM": {
			"openai": {"url", "api_key", "model_name"}, "ollama": {"url", "model_name"},
			"doubao": {"url", "api_key", "model_name"}, "coze": {"bot_id", "user_id", "personal_access_token"},
			"dsh": {"url", "api_key"},
		},
	}
	return required[providerType][subType]
}

func pendingOnboardingDeviceCount() int64 {
	var count int64
	database.GetDB().Model(&models.Device{}).
		Where("agent_id IS NULL AND (user_id IS NULL OR user_id = ?)", database.AdminUserID).
		Count(&count)
	return count
}

func (s *DefaultAdminService) currentOnboardingStatus() onboardingStatus {
	llmName, voice, issues := s.onboardingProviderSetup()
	status := onboardingStatus{
		Ready:          len(issues) == 0,
		Issues:         issues,
		LLM:            llmName,
		Voice:          voice,
		PendingDevices: pendingOnboardingDeviceCount(),
	}
	if agent, err := database.GetOnboardingAgent(database.GetDB()); err == nil {
		status.Exists = true
		status.AgentID = agent.ID
	} else if err != gorm.ErrRecordNotFound {
		status.Issues = append(status.Issues, "Failed to read onboarding assistant status")
		status.Ready = false
	}
	return status
}

func (s *DefaultAdminService) handleOnboardingStatus(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok", "data": s.currentOnboardingStatus()})
}

// handleOnboardingActivate explicitly creates the unique onboarding assistant and takes over previously unbound devices.
func (s *DefaultAdminService) handleOnboardingActivate(c *gin.Context) {
	llmName, voice, issues := s.onboardingProviderSetup()
	if len(issues) > 0 {
		c.JSON(400, gin.H{"status": "error", "message": "Please configure ASR, LLM, and TTS first", "data": gin.H{"issues": issues}})
		return
	}
	tx := database.GetTxDB()
	agent, created, err := database.ActivateOnboardingAgent(tx, llmName, voice)
	if err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"status": "error", "message": "Failed to create onboarding assistant"})
		return
	}
	bound, err := database.BindUnboundDevicesToOnboardingAgent(tx, agent)
	if err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"status": "error", "message": "Failed to bind pending devices"})
		return
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"status": "error", "message": "Failed to save onboarding assistant"})
		return
	}
	message := "Onboarding assistant created"
	if !created {
		message = "Onboarding assistant already exists; current provider configuration synchronized"
	}
	c.JSON(200, gin.H{"status": "ok", "message": message, "data": gin.H{
		"agentId": agent.ID, "created": created, "boundDevices": bound,
	}})
}

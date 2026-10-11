package database

import (
	"fmt"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/models"

	"gorm.io/gorm"
)

const OnboardingAgentName = "Setup Assistant"

const onboardingPrompt = `You are the Setup Assistant. Once an administrator configures local models and initializes you, new devices will automatically bind to you to help users complete setup.
First, briefly greet the user and mention that they can talk to you directly, or ask you to list and switch to existing agents.
When the user explicitly asks to view, create, or update an agent, use the corresponding local tools. Before creating or updating, clarify any missing details.
You can update other agents' names, prompts, LLMs, and voices. Do not delete agents, and do not switch to yourself. Keep answers concise, clear, and natural for voice playback.`

// CreateAgent creates a new Agent (supports transactions)
func CreateAgent(tx *gorm.DB, agent *models.Agent) error {
	if agent == nil {
		return fmt.Errorf("agent cannot be nil")
	}
	agent.UserID = AdminUserID
	return tx.Create(agent).Error
}

func CreateDefaultAgent(tx *gorm.DB, userID uint) (*models.Agent, error) {
	agent := &models.Agent{
		Name:   "Default Agent",
		LLM:    configs.Cfg.SelectedModule["LLM"],
		Voice:  "en-US-AriaNeural",
		UserID: userID,
	}
	err := CreateAgent(tx, agent)
	if err != nil {
		return nil, fmt.Errorf("failed to create default agent: %v", err)
	}
	return agent, nil
}

// GetOnboardingAgent queries the explicit Setup Assistant created by admin.
func GetOnboardingAgent(tx *gorm.DB) (*models.Agent, error) {
	var agent models.Agent
	if err := tx.Where("user_id = ? AND is_onboarding = ?", AdminUserID, true).First(&agent).Error; err != nil {
		return nil, err
	}
	return &agent, nil
}

// ActivateOnboardingAgent creates the single Setup Assistant or synchronizes its LLM and voice.
func ActivateOnboardingAgent(tx *gorm.DB, llmName, voice string) (*models.Agent, bool, error) {
	agent, err := GetOnboardingAgent(tx)
	if err == nil {
		agent.LLM = llmName
		agent.Voice = voice
		if err := UpdateAgent(tx, agent); err != nil {
			return nil, false, fmt.Errorf("failed to sync setup assistant configuration: %w", err)
		}
		return agent, false, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, false, fmt.Errorf("failed to query setup assistant: %w", err)
	}

	agent = &models.Agent{
		Name:         OnboardingAgentName,
		LLM:          llmName,
		Language:     "English",
		Voice:        voice,
		Prompt:       onboardingPrompt,
		UserID:       AdminUserID,
		EnabledTools: "switch_agent,list_agents,create_agent,update_agent",
		Description:  "Assists new devices with setup and manages other agents.",
		IsOnboarding: true,
	}
	if err := CreateAgent(tx, agent); err != nil {
		return nil, false, fmt.Errorf("failed to create setup assistant: %w", err)
	}
	return agent, true, nil
}

// BindUnboundDevicesToOnboardingAgent binds pending devices to the Setup Assistant.
func BindUnboundDevicesToOnboardingAgent(tx *gorm.DB, agent *models.Agent) (int64, error) {
	if agent == nil || !agent.IsOnboarding {
		return 0, fmt.Errorf("invalid setup assistant")
	}
	result := tx.Model(&models.Device{}).
		Where("agent_id IS NULL AND (user_id IS NULL OR user_id = ?)", AdminUserID).
		Updates(map[string]interface{}{
			"agent_id":    agent.ID,
			"user_id":     agent.UserID,
			"auth_status": "onboarding",
		})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to bind pending devices: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func IsOnboardingAgent(tx *gorm.DB, id uint) bool {
	if id == 0 {
		return false
	}
	var count int64
	return tx.Model(&models.Agent{}).Where("id = ? AND user_id = ? AND is_onboarding = ?", id, AdminUserID, true).Count(&count).Error == nil && count == 1
}

// ListAgentsByUser returns all agents for a user.
func ListAgentsByUser(tx *gorm.DB, userID uint) ([]models.Agent, error) {
	var agents []models.Agent
	err := tx.Where("user_id = ?", userID).Preload("Devices").Find(&agents).Error
	return agents, err
}

// GetAgentByID returns a single agent by ID.
func GetAgentByID(tx *gorm.DB, id uint) (*models.Agent, error) {
	var agent models.Agent
	err := tx.Where("id = ?", id).First(&agent).Error
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

// GetAgentByIDAndUser returns an agent for a specific user.
func GetAgentByIDAndUser(tx *gorm.DB, id uint, userID uint) (*models.Agent, error) {
	var agent models.Agent
	err := tx.Where("id = ? AND user_id = ?", id, userID).First(&agent).Error
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

// UpdateAgent updates agent attributes.
func UpdateAgent(tx *gorm.DB, agent *models.Agent) error {
	return tx.Model(agent).Updates(agent).Error
}

// DeleteAgent deletes an agent after verifying no bound devices remain.
func DeleteAgent(tx *gorm.DB, id uint, userID uint) error {
	if IsOnboardingAgent(tx, id) {
		return fmt.Errorf("setup assistant is a system agent and cannot be deleted")
	}
	if err := tx.Unscoped().Model(&models.Device{}).
		Where("agent_id = ? AND deleted_at IS NOT NULL", id).
		Update("agent_id", nil).Error; err != nil {
		return fmt.Errorf("failed to clean up agent reference on deleted devices: %v", err)
	}

	var deviceCount int64
	err := tx.Model(&models.Device{}).Where("agent_id = ?", id).Count(&deviceCount).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("agent not found or already deleted")
		}
		dbLogger.Error("failed to query devices bound to agent: %v", err)
		return fmt.Errorf("operation failed, please contact administrator")
	}
	if deviceCount > 0 {
		return fmt.Errorf("please unbind devices before deleting agent")
	}

	result := tx.Where("id = ? AND user_id = ?", id, userID).Delete(&models.Agent{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete agent: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("agent not found or already deleted")
	}
	return nil
}

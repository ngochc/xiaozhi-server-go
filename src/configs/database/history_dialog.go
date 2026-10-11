package database

import (
	"fmt"
	"time"
	"xiaozhi-server-go/src/models"

	"gorm.io/gorm"
)

// SaveAgentDialog saves agent conversation history (supports transactions)
func SaveAgentDialog(
	tx *gorm.DB,
	AgentID uint,
	userID uint,
	dialogStr string,
	conversationid string,
) error {
	var existingDialog models.AgentDialog
	err := tx.Where("agent_id = ? AND conversationid = ?", AgentID, conversationid).
		First(&existingDialog).
		Error
	if err == nil {
		existingDialog.Dialog = dialogStr
		existingDialog.UpdatedAt = time.Now()
		return tx.Save(&existingDialog).Error
	}

	agentDialog := &models.AgentDialog{
		AgentID:        AgentID,
		UserID:         AdminUserID,
		Dialog:         dialogStr,
		Conversationid: conversationid,
		CreatedAt:      time.Now(),
	}
	return tx.Create(agentDialog).Error
}

func GetAgentDialogByConversationID(
	tx *gorm.DB,
	AgentID uint,
	conversationid string,
) (*models.AgentDialog, error) {
	var agentDialog models.AgentDialog
	err := tx.Where("agent_id = ? AND conversationid = ?", AgentID, conversationid).
		First(&agentDialog).
		Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("dialog record not found")
		}
		return nil, fmt.Errorf("failed to query dialog record: %v", err)
	}
	return &agentDialog, nil
}

func GetAgentDialogByID(
	tx *gorm.DB,
	id uint,
) (*models.AgentDialog, error) {
	var agentDialog models.AgentDialog
	if err := tx.Where("id = ?", id).First(&agentDialog).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("dialog record not found")
		}
		return nil, fmt.Errorf("failed to query dialog record: %v", err)
	}
	return &agentDialog, nil
}

func GetAgentDialogsWithoutDetailByID(
	tx *gorm.DB,
	AgentID uint,
) ([]models.AgentDialog, error) {
	var agentDialogs []models.AgentDialog
	err := tx.Where("agent_id = ?", AgentID).
		Order("created_at DESC").
		Omit("Dialog").
		Find(&agentDialogs).
		Error
	if err != nil {
		return nil, fmt.Errorf("failed to query dialog records: %v", err)
	}
	return agentDialogs, nil
}

func DeleteAgentDialogByID(
	tx *gorm.DB,
	id uint,
) error {
	var agentDialog models.AgentDialog
	if err := tx.Where("id = ?", id).First(&agentDialog).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("dialog record not found")
		}
		return fmt.Errorf("failed to query dialog record: %v", err)
	}
	if err := tx.Delete(&agentDialog).Error; err != nil {
		return fmt.Errorf("failed to delete dialog record: %v", err)
	}
	return nil
}

func SaveAgentConversation(tx *gorm.DB, agentID uint, conversationID string) error {
	return tx.Model(&models.Agent{}).
		Where("id = ?", agentID).
		Updates(map[string]interface{}{
			"conversationid":       conversationID,
			"last_conversation_at": time.Now(),
		}).Error
}

func ClearAgentConversation(tx *gorm.DB, agentID uint) error {
	return tx.Model(&models.Agent{}).Where("id = ?", agentID).Update("conversationid", "").Error
}

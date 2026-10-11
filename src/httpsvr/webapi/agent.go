package webapi

import (
	"net/http"
	"strconv"
	"time"
	"xiaozhi-server-go/src/configs/database"
	"xiaozhi-server-go/src/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CreateAgentRequest represents Agent creation payload
type AgentCreateRequest struct {
	Prompt     string `json:"prompt"`
	Name       string `json:"name"` // Agent name
	LLM        string `json:"LLM"`
	Language   string `json:"language"`   // Language
	Voice      string `json:"voice"`      // Voice，Default voice identifier
	VoiceName  string `json:"voiceName"`  // Voice display name
	ASRSpeed   int    `json:"asrSpeed"`   // ASR speed: 1=patient, 2=normal, 3=fast
	SpeakSpeed int    `json:"speakSpeed"` // TTS rate: 1=slow, 2=normal, 3=fast
	Tone       int    `json:"tone"`       // TTS pitch: 1-100, low to high
}

// handleAgentCreate Create agent request body
// @Summary Create new agent
// @Description Create new agent
// @Tags Agent
// @Accept json
// @Produce json
// @Param data body AgentCreateRequest true "Agent creation parameters"
// @Success 200 {object} models.Agent "Agent information returned on successful creation"
// @Router /user/agent/create [post]
func (s *DefaultUserService) handleAgentCreate(c *gin.Context) {
	userID := currentUserID(c)
	var req AgentCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	agent := &models.Agent{
		Prompt:     req.Prompt,
		Name:       req.Name,
		LLM:        req.LLM,
		Language:   req.Language,
		Voice:      req.Voice,
		VoiceName:  req.VoiceName,
		ASRSpeed:   req.ASRSpeed,
		SpeakSpeed: req.SpeakSpeed,
		Tone:       req.Tone,
		UserID:     userID,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	WithTx(c, func(tx *gorm.DB) error {
		if err := database.CreateAgent(tx, agent); err != nil {
			return err
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "data": agent})
		return nil
	})
}

// Construct response struct with device id list
type AgentWithDeviceIDs struct {
	models.Agent
	DeviceIDs []uint `json:"deviceIDs"`
}

// handleAgentList Get agent list
// @Summary Get all agents for the current user
// @Description Get all agents and their device ID lists for the current user
// @Tags Agent
// @Produce json
// @Success 200 {object} []AgentWithDeviceIDs "Agent list"
// @Router /user/agent/list [get]
func (s *DefaultUserService) handleAgentList(c *gin.Context) {
	userID := currentUserID(c)
	WithTx(c, func(tx *gorm.DB) error {
		agents, err := database.ListAgentsByUser(tx, userID)
		if err != nil {
			return err
		}
		var result []AgentWithDeviceIDs
		for _, agent := range agents {
			var ids []uint
			for _, d := range agent.Devices {
				ids = append(ids, d.ID)
			}
			result = append(result, AgentWithDeviceIDs{
				Agent:     agent,
				DeviceIDs: ids,
			})
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "data": result})
		return nil
	})
}

// handleAgentGet Get single agent
// @Summary Get agent by ID
// @Description Get agent details by ID
// @Tags Agent
// @Produce json
// @Param id path int true "Agent ID"
// @Success 200 {object} models.Agent "Agent information"
// @Router /user/agent/{id} [get]
func (s *DefaultUserService) handleAgentGet(c *gin.Context) {
	userID := currentUserID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	WithTx(c, func(tx *gorm.DB) error {
		agent, err := database.GetAgentByIDAndUser(tx, uint(id), userID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return err
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "data": agent})
		return nil
	})
}

// handleAgentUpdate Update agent
// @Summary Update agent by ID
// @Description Update agent information by ID
// @Tags Agent
// @Accept json
// @Produce json
// @Param id path int true "Agent ID"
// @Param data body AgentCreateRequest true "Agent update parameters"
// @Success 200 {object} models.Agent "Updated agent information"
// @Router /user/agent/{id} [put]
func (s *DefaultUserService) handleAgentUpdate(c *gin.Context) {
	userID := currentUserID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req AgentCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	WithTx(c, func(tx *gorm.DB) error {
		agent, err := database.GetAgentByIDAndUser(tx, uint(id), userID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return err
		}

		agent.Prompt = req.Prompt
		agent.Name = req.Name
		agent.LLM = req.LLM
		agent.Language = req.Language
		agent.Voice = req.Voice
		agent.VoiceName = req.VoiceName
		agent.ASRSpeed = req.ASRSpeed
		agent.SpeakSpeed = req.SpeakSpeed
		agent.Tone = req.Tone
		agent.UpdatedAt = time.Now() // Update modification time
		if agent.CreatedAt.IsZero() {
			agent.CreatedAt = time.Now() // If creation time is not set, set to current time
		}

		if err := database.UpdateAgent(tx, agent); err != nil {
			return err
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "data": agent})
		return nil
	})
}

// handleAgentDelete Delete agent
// @Summary Delete agent by ID
// @Description Delete agent by ID
// @Tags Agent
// @Produce json
// @Param id path int true "Agent ID"
// @Success 200 {object} map[string]interface{} "Deletion result"
// @Router /user/agent/{id} [delete]
func (s *DefaultUserService) handleAgentDelete(c *gin.Context) {
	userID := currentUserID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		s.logger.Error("Invalid Agent ID: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	WithTx(c, func(tx *gorm.DB) error {
		if err := database.DeleteAgent(tx, uint(id), userID); err != nil {
			s.logger.Error("Failed to delete Agent: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return err
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return nil
	})
}

// handleAgentHistoryDialogList Get agent dialog history list
// @Summary Get agent dialog history list
// @Description Get dialog history list for specified agent
// @Tags Agent
// @Produce json
// @Param id path int true "Agent ID"
// @Success 200 {object} []models.AgentDialog "Dialog history list"
// @Router /user/agent/history_dialog_list/{id} [post]
func (s *DefaultUserService) handleAgentHistoryDialogList(c *gin.Context) {
	userID := currentUserID(c)
	agentID := c.Param("id")
	id, err := strconv.Atoi(agentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}
	WithTx(c, func(tx *gorm.DB) error {
		if _, err := database.GetAgentByIDAndUser(tx, uint(id), userID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return err
		}
		dialogs, err := database.GetAgentDialogsWithoutDetailByID(tx, uint(id))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return err
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "data": dialogs})
		return nil
	})
}

// handleAgentGetHistoryDialog Get single agent dialog record
// @Summary Get single agent dialog record
// @Description Get single agent dialog record by dialog ID
// @Tags Agent
// @Produce json
// @Param dialog_id path int true "Dialog ID"
// @Success 200 {object} models.AgentDialog "Dialog record"
// @Router /user/agent/history_dialog/{dialog_id} [get]
func (s *DefaultUserService) handleAgentGetHistoryDialog(c *gin.Context) {
	dialogID := c.Param("dialog_id")
	id, err := strconv.Atoi(dialogID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dialog id"})
		return
	}
	WithTx(c, func(tx *gorm.DB) error {
		dialog, err := database.GetAgentDialogByID(tx, uint(id))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "dialog not found"})
			return err
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "data": dialog})
		return nil
	})
}

// handleAgentDeleteHistoryDialog Delete single agent dialog record
// @Summary Delete single agent dialog record
// @Description Delete single agent dialog record by dialog ID
// @Tags Agent
// @Produce json
// @Param dialog_id path int true "Dialog ID"
// @Success 200 {object} map[string]interface{} "Deletion result"
// @Router /user/agent/history_dialog/{dialog_id} [delete]
func (s *DefaultUserService) handleAgentDeleteHistoryDialog(c *gin.Context) {
	dialogID := c.Param("dialog_id")
	id, err := strconv.Atoi(dialogID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dialog id"})
		return
	}
	WithTx(c, func(tx *gorm.DB) error {
		if err := database.DeleteAgentDialogByID(tx, uint(id)); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "dialog not found"})
			return err
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return nil
	})
}

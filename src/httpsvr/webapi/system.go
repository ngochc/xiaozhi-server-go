package webapi

import (
	"context"
	"encoding/json"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/configs/database"
	"xiaozhi-server-go/src/core/utils"

	"github.com/gin-gonic/gin"
)

type DefaultAdminService struct {
	logger *utils.Logger
	config *configs.Config
}

// NewDefaultAdminService constructor
func NewDefaultAdminService(
	config *configs.Config,
	logger *utils.Logger,
) (*DefaultAdminService, error) {
	service := &DefaultAdminService{
		logger: logger,
		config: config,
	}

	return service, nil
}

// Start registers Admin service routes
func (s *DefaultAdminService) Start(
	ctx context.Context,
	engine *gin.Engine,
	apiGroup *gin.RouterGroup,
) error {
	apiGroup.GET("/admin", s.handleGet)

	// Protected routes requiring admin permissions
	adminGroup := apiGroup.Group("")
	// Public read access for model inspection
	adminGroup.Use(AuthMiddleware())
	{
		adminGroup.GET("/admin/system", s.handleSystemGet)
		adminGroup.GET("/admin/system/providers/:type", s.handleSystemProvidersType)
	}
	adminGroup.Use(AuthMiddleware(), AdminMiddleware())
	{
		adminGroup.POST("/admin/system", s.handleSystemPost)
		adminGroup.GET("/admin/onboarding", s.handleOnboardingStatus)
		adminGroup.POST("/admin/onboarding", s.handleOnboardingActivate)

		adminGroup.DELETE("/admin/system/device", s.handleDeviceDeleteAdmin)
		// providers
		adminGroup.GET("/admin/system/providers", s.handleSystemProvidersGet)
		adminGroup.GET("/admin/system/providers/:type/:name", s.handleSystemProvidersGetByName)

		adminGroup.POST("/admin/system/providers/create", s.handleSystemProvidersCreate)
		adminGroup.PUT("/admin/system/providers/:type/:name", s.handleSystemProvidersUpdate)
		adminGroup.DELETE("/admin/system/providers/:type/:name", s.handleSystemProvidersDelete)
	}

	s.logger.Info("Admin HTTP routes registered successfully")
	return nil
}

func (s *DefaultAdminService) handleGet(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Admin service is running",
	})
}

type SystemConfig struct {
	SelectedASR     string   `                  json:"selectedASR"`
	SelectedTTS     string   `                  json:"selectedTTS"`
	SelectedLLM     string   `                  json:"selectedLLM"`
	SelectedVLLLM   string   `                  json:"selectedVLLLM"`
	Prompt          string   ` json:"prompt"`
	QuickReplyWords []string `                  json:"quickReplyWords"` // Stored as JSON array
}

// handleSystemGet retrieves system configuration
// @Summary Retrieve system configuration
// @Description Get current system configuration
// @Tags System
// @Produce json
// @Success 200 {object} map[string]interface{} "System configuration information"
// @Router /admin/system [get]
func (s *DefaultAdminService) handleSystemGet(c *gin.Context) {
	var config SystemConfig
	config.SelectedASR = configs.Cfg.SelectedModule["ASR"]
	config.SelectedTTS = configs.Cfg.SelectedModule["TTS"]
	config.SelectedLLM = configs.Cfg.SelectedModule["LLM"]
	config.SelectedVLLLM = configs.Cfg.SelectedModule["VLLLM"]
	config.Prompt = configs.Cfg.DefaultPrompt
	config.QuickReplyWords = configs.Cfg.QuickReplyWords

	var data map[string]interface{}
	tmp, _ := json.Marshal(config)
	json.Unmarshal(tmp, &data)

	asrList, ttsList, llmList, vllmList := database.GetProviderNameList(database.AdminUserID)
	data["asrList"] = asrList
	data["llmList"] = llmList
	data["ttsList"] = ttsList
	data["vllmList"] = vllmList

	// fmt.Println("JSON string:", data)
	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "System configuration retrieved successfully",
		"data":    data,
	})
}

// handleSystemGet retrieves system configuration
// @Summary Retrieve system configuration
// @Description Get current system configuration
// @Tags System
// @Produce json
// @Success 200 {object} map[string]interface{} "System configuration information"
// @Router /admin/system [get]
func (s *DefaultAdminService) handleSystemPost(c *gin.Context) {
	// Define request struct
	var requestData struct {
		Data string `json:"data"`
	}

	// Bind JSON data to struct
	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid JSON format",
			"error":   err.Error(),
		})
		return
	}

	// Check if data field is empty
	if requestData.Data == "" {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Missing 'data' field in request body",
			"error":   "data field is required",
		})
		return
	}

	s.logger.Info("Received system configuration data: %s", requestData.Data)

	// Parse JSON string in data field into SystemConfig struct
	var config SystemConfig
	if err := json.Unmarshal([]byte(requestData.Data), &config); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid system configuration data",
			"error":   err.Error(),
		})
		return
	}

	configs.Cfg.SelectedModule["ASR"] = config.SelectedASR
	configs.Cfg.SelectedModule["TTS"] = config.SelectedTTS
	configs.Cfg.SelectedModule["LLM"] = config.SelectedLLM
	configs.Cfg.SelectedModule["VLLM"] = config.SelectedVLLLM
	configs.Cfg.DefaultPrompt = config.Prompt
	configs.Cfg.QuickReplyWords = config.QuickReplyWords

	configs.Cfg.SaveToDB(database.GetServerConfigDB())
	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "System configuration saved successfully",
	})
}

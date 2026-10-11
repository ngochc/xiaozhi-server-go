package webapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"xiaozhi-server-go/src/configs/database"

	"github.com/gin-gonic/gin"
)

// handleUserProviderTest validates saved provider configurations.
// Verifies live response from provider.
func (s *DefaultUserService) handleUserProviderTest(c *gin.Context) {
	providerType := strings.ToUpper(strings.TrimSpace(c.Param("type")))
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		c.JSON(400, gin.H{"success": false, "message": "Provider name cannot be empty"})
		return
	}

	providers, err := database.GetProviderByTypeInternal(providerType, currentUserID(c), false)
	if err != nil {
		c.JSON(400, gin.H{"success": false, "message": err.Error()})
		return
	}
	rawConfig, ok := providers[name]
	if !ok {
		c.JSON(404, gin.H{"success": false, "message": "Provider not found for testing"})
		return
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(rawConfig), &config); err != nil {
		c.JSON(400, gin.H{"success": false, "message": "Provider config is not valid JSON"})
		return
	}
	subType, _ := config["type"].(string)
	subType = strings.ToLower(strings.TrimSpace(subType))
	if subType == "" {
		c.JSON(200, gin.H{"success": false, "message": "Missing specific subtype (type)"})
		return
	}
	if missing := missingProviderTestFields(providerType, subType, config); len(missing) > 0 {
		c.JSON(200, gin.H{
			"success": false,
			"message": fmt.Sprintf("Required fields missing: %s", strings.Join(missing, "、")),
			"data":    gin.H{"level": "configuration", "missing_fields": missing},
		})
		return
	}

	result, err := s.runLiveProviderTest(c.Request.Context(), providerType, name, subType, config)
	if err != nil {
		c.JSON(200, gin.H{"success": false, "message": err.Error(), "data": gin.H{"level": "live"}})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": result.message,
		"data":    result.data,
	})
}

func stringConfig(config map[string]interface{}, key string) string {
	value, _ := config[key].(string)
	return strings.TrimSpace(value)
}

func missingProviderTestFields(providerType, subType string, config map[string]interface{}) []string {
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
		"VLLLM": {
			"openai": {"url", "api_key", "model_name"}, "ollama": {"url", "model_name"},
		},
	}
	fields, known := required[providerType][subType]
	if !known {
		return []string{"testing is not supported for this provider type"}
	}
	missing := make([]string, 0)
	for _, field := range fields {
		if stringConfig(config, field) == "" {
			missing = append(missing, field)
		}
	}
	return missing
}

// handleSystemProvidersGet retrieves all providers
// @Summary Get all providers
// @Description Get all provider information in the system
// @Tags Provider
// @Produce json
// @Success 200 {object} []interface{} "Provider list"
// @Router /admin/system/providers [get]
func (s *DefaultAdminService) handleSystemProvidersGet(c *gin.Context) {
	providers := database.GetAllProviders(database.AdminUserID)
	if len(providers) == 0 {
		c.JSON(404, gin.H{
			"status":  "error",
			"message": "No providers found",
		})
		return
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Providers retrieved successfully",
		"data":    providers,
	})
}

// handleSystemProvidersType Get providers of specified type [deprecated, use /user/providers/{type}]
// @Summary Get providers of specified type
// @Description Get provider information by type
// @Tags Provider
// @Produce json
// @Param type path string true "Provider type"
// @Success 200 {object} interface{} "Provider information"
// @Router /admin/system/providers/{type} [get]
func (s *DefaultAdminService) handleSystemProvidersType(c *gin.Context) {
	providerType := c.Param("type")
	if providerType == "" {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Provider type is required",
		})
		return
	}

	provider, err := database.GetProviderByType(providerType, database.AdminUserID)
	if err != nil {
		c.JSON(404, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Provider not found for type: %s", providerType),
			"error":   err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": fmt.Sprintf("Provider for type %s retrieved successfully", providerType),
		"data":    provider,
	})
}

// handleSystemProvidersGetByName Get provider by type and name
// @Summary Get provider by type and name
// @Description Get provider information by type and name
// @Tags Provider
// @Produce json
// @Param type path string true "Provider type"
// @Param name path string true "Provider name"
// @Success 200 {object} interface{} "Provider information"
// @Router /admin/system/providers/{type}/{name} [get]
func (s *DefaultAdminService) handleSystemProvidersGetByName(c *gin.Context) {
	providerType := c.Param("type")
	name := c.Param("name")

	if providerType == "" || name == "" {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Provider type and name are required",
		})
		return
	}

	provider, err := database.GetProviderByName(providerType, name)
	if err != nil {
		c.JSON(404, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Provider not found: %s/%s", providerType, name),
			"error":   err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": fmt.Sprintf("Provider %s/%s retrieved successfully", providerType, name),
		"data":    provider,
	})
}

// handleSystemProvidersCreate Create provider [deprecated, use /user/providers/create]
// @Summary Create provider
// @Description Create new provider
// @Tags Provider
// @Accept json
// @Produce json
// @Param data body object true "Provider creation parameters"
// @Success 201 {object} map[string]interface{} "Creation result"
// @Router /user/providers/create [post]
func (s *DefaultAdminService) handleSystemProvidersCreate(c *gin.Context) {
	userID, _ := c.Get("user_id")
	user, err := database.GetUserByID(database.GetDB(), userID.(uint))
	if err != nil || user == nil {
		c.JSON(404, gin.H{
			"status":  "error",
			"message": "User does not exist",
		})
		return
	}
	createUserID := user.ID
	if user.Role == "admin" {
		createUserID = 1
	}

	var requestData struct {
		Type string      `json:"type" binding:"required"`
		Name string      `json:"name" binding:"required"`
		Data interface{} `json:"data" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid request data",
			"error":   err.Error(),
		})
		return
	}

	// Check if a provider with the same name already exists
	existingProvider, err := database.GetProviderByName(requestData.Type, requestData.Name)
	if err == nil && existingProvider != "" {
		c.JSON(409, gin.H{
			"status": "error",
			"message": fmt.Sprintf(
				"Provider with name '%s' already exists for type '%s'",
				requestData.Name,
				requestData.Type,
			),
			"error": "duplicate_provider_name",
		})
		return
	}

	s.logger.Info("Creating new provider: type=%s, name=%s", requestData.Type, requestData.Name)

	if err := database.CreateProvider(requestData.Type, requestData.Name, requestData.Data, createUserID); err != nil {
		c.JSON(500, gin.H{
			"status":  "error",
			"message": "Failed to create provider",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(201, gin.H{
		"status": "ok",
		"message": fmt.Sprintf(
			"Provider %s/%s created successfully",
			requestData.Type,
			requestData.Name,
		),
	})
}

// handleSystemProvidersUpdate Update provider [deprecated, use /user/providers/{type}/{name}]
// @Summary Update provider
// @Description Update provider of specified type and name
// @Tags Provider
// @Accept json
// @Produce json
// @Param type path string true "Provider type"
// @Param name path string true "Provider name"
// @Param data body object true "Provider update parameters"
// @Success 200 {object} map[string]interface{} "Update result"
// @Router /admin/system/providers/{type}/{name} [put]
func (s *DefaultAdminService) handleSystemProvidersUpdate(c *gin.Context) {
	providerType := c.Param("type")
	name := c.Param("name")

	if providerType == "" || name == "" {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Provider type and name are required",
		})
		return
	}

	var requestData struct {
		Data interface{} `json:"data" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid request data",
			"error":   err.Error(),
		})
		return
	}

	s.logger.Info("Updating provider: type=%s, name=%s", providerType, name)

	if err := database.UpdateProvider(providerType, name, requestData.Data, database.AdminUserID); err != nil {
		c.JSON(500, gin.H{
			"status":  "error",
			"message": "Failed to update provider",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": fmt.Sprintf("Provider %s/%s updated successfully", providerType, name),
	})
}

// handleSystemProvidersDelete Delete provider [deprecated, use /user/providers/{type}/{name}]
// @Summary Delete provider
// @Description Delete provider of specified type and name
// @Tags Provider
// @Produce json
// @Param type path string true "Provider type"
// @Param name path string true "Provider name"
// @Success 200 {object} map[string]interface{} "Deletion result"
// @Router /admin/system/providers/{type}/{name} [delete]
func (s *DefaultAdminService) handleSystemProvidersDelete(c *gin.Context) {
	providerType := c.Param("type")
	name := c.Param("name")

	if providerType == "" || name == "" {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Provider type and name are required",
		})
		return
	}

	s.logger.Info("Deleting provider: type=%s, name=%s", providerType, name)

	if err := database.DeleteProvider(providerType, name, database.AdminUserID); err != nil {
		c.JSON(500, gin.H{
			"status":  "error",
			"message": "Failed to delete provider",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": fmt.Sprintf("Provider %s/%s deleted successfully", providerType, name),
	})
}

//=============================================================================== user providers ==============

// handleUserProvidersType Get providers of specified type
// @Summary Get providers of specified type
// @Description Get provider information by type
// @Tags Provider
// @Produce json
// @Param type path string true "Provider type"
// @Success 200 {object} interface{} "Provider information"
// @Router /user/providers/{type} [get]
func (s *DefaultUserService) handleUserProvidersType(c *gin.Context) {
	userID, _ := c.Get("user_id")
	providerType := c.Param("type")
	if providerType == "" {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Provider type is required",
		})
		return
	}
	user, err := database.GetUserByID(database.GetDB(), userID.(uint))
	if err != nil || user == nil {
		c.JSON(404, gin.H{
			"status":  "error",
			"message": "User does not exist",
		})
		return
	}

	provider, err := database.GetProviderByTypeInternal(providerType, userID.(uint), (user.Role != "admin"))
	if err != nil {
		c.JSON(404, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Provider not found for type: %s", providerType),
			"error":   err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": fmt.Sprintf("Provider for type %s retrieved successfully", providerType),
		"data":    provider,
	})
}

// handleUserProvidersCreate User creates provider
// @Summary User creates provider
// @Description User creates new provider; created provider is private to the user
// @Tags Provider
// @Accept json
// @Produce json
// @Param data body object true "Provider creation parameters"
// @Success 201 {object} map[string]interface{} "Creation result"
// @Router /user/providers/create [post]
func (s *DefaultUserService) handleUserProvidersCreate(c *gin.Context) {
	userID, _ := c.Get("user_id")
	user, err := database.GetUserByID(database.GetDB(), userID.(uint))
	if err != nil || user == nil {
		c.JSON(404, gin.H{
			"status":  "error",
			"message": "User does not exist",
		})
		return
	}
	createUserID := user.ID
	if user.Role == "admin" {
		createUserID = 1
	}

	var requestData struct {
		Type string      `json:"type" binding:"required"`
		Name string      `json:"name" binding:"required"`
		Data interface{} `json:"data" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid request data",
			"error":   err.Error(),
		})
		return
	}

	// Check if a provider with the same name already exists
	existingProvider, err := database.GetProviderByName(requestData.Type, requestData.Name)
	if err == nil && existingProvider != "" {
		c.JSON(409, gin.H{
			"status": "error",
			"message": fmt.Sprintf(
				"Provider with name '%s' already exists for type '%s'",
				requestData.Name,
				requestData.Type,
			),
			"error": "duplicate_provider_name",
		})
		return
	}

	s.logger.Info("Creating new provider: type=%s, name=%s", requestData.Type, requestData.Name)

	if err := database.CreateProvider(requestData.Type, requestData.Name, requestData.Data, createUserID); err != nil {
		c.JSON(500, gin.H{
			"status":  "error",
			"message": "Failed to create provider",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(201, gin.H{
		"status": "ok",
		"message": fmt.Sprintf(
			"Provider %s/%s created successfully",
			requestData.Type,
			requestData.Name,
		),
	})
}

// handleUserProvidersDelete Delete provider
// @Summary Delete provider
// @Description Delete provider of specified type and name, only allowed for user-created providers
// @Tags Provider
// @Produce json
// @Param type path string true "Provider type"
// @Param name path string true "Provider name"
// @Success 200 {object} map[string]interface{} "Deletion result"
// @Router /user/providers/{type}/{name} [delete]
func (s *DefaultUserService) handleUserProvidersDelete(c *gin.Context) {
	userID, _ := c.Get("user_id")
	user, err := database.GetUserByID(database.GetDB(), userID.(uint))
	if err != nil || user == nil {
		c.JSON(404, gin.H{
			"status":  "error",
			"message": "User does not exist",
		})
		return
	}

	providerType := c.Param("type")
	name := c.Param("name")

	if providerType == "" || name == "" {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Provider type and name are required",
		})
		return
	}
	deleteUserID := user.ID
	if user.Role == "admin" {
		deleteUserID = database.AdminUserID
	}

	if err := database.DeleteProvider(providerType, name, deleteUserID); err != nil {
		s.logger.Error("Failed to delete provider: %s/%s, error: %v", providerType, name, err)
		// Check if error indicates permission denied
		if errors.Is(err, database.ErrNoPermission) {
			c.JSON(403, gin.H{
				"status":  "error",
				"message": "Permission denied to delete this provider",
			})
			return
		}
		c.JSON(500, gin.H{
			"status":  "error",
			"message": "Failed to delete provider",
			"error":   err.Error(),
		})
		return
	}

	s.logger.Info("Deleting provider: type=%s, name=%s, user_id=%d, userName=%s, role=%s", providerType, name, user.ID, user.Username, user.Role)
	c.JSON(200, gin.H{
		"status":  "ok",
		"message": fmt.Sprintf("Provider %s/%s deleted successfully", providerType, name),
	})
}

// handleUserProvidersUpdate Update provider
// @Summary Update provider
// @Description Update provider of specified type and name, only allowed for user-created providers
// @Tags Provider
// @Accept json
// @Produce json
// @Param type path string true "Provider type"
// @Param name path string true "Provider name"
// @Param data body object true "Provider update parameters"
// @Success 200 {object} map[string]interface{} "Update result"
// @Router /user/providers/{type}/{name} [put]
func (s *DefaultUserService) handleUserProvidersUpdate(c *gin.Context) {
	userID, _ := c.Get("user_id")
	user, err := database.GetUserByID(database.GetDB(), userID.(uint))
	if err != nil || user == nil {
		c.JSON(404, gin.H{
			"status":  "error",
			"message": "User does not exist",
		})
		return
	}

	providerType := c.Param("type")
	name := c.Param("name")

	if providerType == "" || name == "" {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Provider type and name are required",
		})
		return
	}

	var requestData struct {
		Data interface{} `json:"data" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(400, gin.H{
			"status":  "error",
			"message": "Invalid request data",
			"error":   err.Error(),
		})
		return
	}

	updataUserID := user.ID
	if user.Role == "admin" {
		updataUserID = 1
	}

	s.logger.Info("Updating provider: type=%s, name=%s, user_id=%d", providerType, name, user.ID)

	if err := database.UpdateProvider(providerType, name, requestData.Data, updataUserID); err != nil {
		c.JSON(500, gin.H{
			"status":  "error",
			"message": "Failed to update provider",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": fmt.Sprintf("Provider %s/%s updated successfully", providerType, name),
	})
}

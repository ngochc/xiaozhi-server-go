package vision

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/auth"
	"xiaozhi-server-go/src/core/image"
	"xiaozhi-server-go/src/core/providers"
	"xiaozhi-server-go/src/core/providers/vlllm"
	"xiaozhi-server-go/src/core/utils"

	"github.com/gin-gonic/gin"
)

const (
	// Max file size 5MB
	MAX_FILE_SIZE = 5 * 1024 * 1024
)

type DefaultVisionService struct {
	logger    *utils.Logger
	config    *configs.Config
	vlllmMap  map[string]*vlllm.Provider // Supports multiple VLLM providers
	authToken *auth.AuthToken            // Authentication helper
}

// NewDefaultVisionService constructor
func NewDefaultVisionService(
	config *configs.Config,
	logger *utils.Logger,
) (*DefaultVisionService, error) {
	service := &DefaultVisionService{
		logger:   logger,
		config:   config,
		vlllmMap: make(map[string]*vlllm.Provider),
	}

	service.authToken = auth.NewAuthToken(config.Server.Token)

	// Initialize VLLM providers
	if err := service.initVLLMProviders(); err != nil {
		return nil, fmt.Errorf("failed to initialize VLLM providers: %v", err)
	}

	return service, nil
}

// initVLLMProviders initializes VLLM providers
func (s *DefaultVisionService) initVLLMProviders() error {
	// Check configured VLLM provider
	selected_vlllm := s.config.SelectedModule["VLLLM"]
	if selected_vlllm == "" {
		s.logger.Warn("Please configure VLLM provider")
		return fmt.Errorf("please configure VLLM provider")
	}

	vlllmConfig := s.config.VLLLM[selected_vlllm]

	// Create VLLM provider configuration
	providerConfig := &vlllm.Config{
		Type:        vlllmConfig.Type,
		ModelName:   vlllmConfig.ModelName,
		BaseURL:     vlllmConfig.BaseURL,
		APIKey:      vlllmConfig.APIKey,
		Temperature: vlllmConfig.Temperature,
		MaxTokens:   vlllmConfig.MaxTokens,
		TopP:        vlllmConfig.TopP,
		Security:    vlllmConfig.Security,
	}

	// Create provider instance
	provider, err := vlllm.NewProvider(providerConfig, s.logger)
	if err != nil {
		s.logger.Warn(fmt.Sprintf("Failed to create VLLM provider: %v", err))
	}

	// Initialize provider
	if err := provider.Initialize(); err != nil {
		s.logger.Warn(fmt.Sprintf("Failed to initialize VLLM provider: %v", err))
	}

	s.vlllmMap[selected_vlllm] = provider
	s.logger.Info(fmt.Sprintf("[VLLM] [Initialized %s] successfully", selected_vlllm))

	if len(s.vlllmMap) == 0 {
		s.logger.Error("No available VLLM provider, please check configuration")
		return fmt.Errorf("no available VLLM provider")
	}

	return nil
}

// Start registers Vision endpoints
func (s *DefaultVisionService) Start(
	ctx context.Context,
	engine *gin.Engine,
	apiGroup *gin.RouterGroup,
) error {
	// Vision main endpoints (GET for status check, POST for image analysis)
	apiGroup.GET("/vision", s.handleGet)
	apiGroup.POST("/vision", s.handlePost)
	apiGroup.OPTIONS("/vision", s.handleOptions)

	s.logger.Info("[Vision] [Service] HTTP routes registered successfully")
	return nil
}

// handleOptions handles OPTIONS requests (CORS)
func (s *DefaultVisionService) handleOptions(c *gin.Context) {
	s.logger.Info("Received Vision CORS preflight OPTIONS request")
	s.addCORSHeaders(c)
	c.Status(http.StatusOK)
}

// handleGet handles GET status check request
// @Summary Vision service status check
// @Description Check if Vision service is running normally
// @Tags Vision
// @Produce plain
// @Success 200 {string} string "Service status message"
// @Router /vision [get]
func (s *DefaultVisionService) handleGet(c *gin.Context) {
	s.logger.Info("Received Vision status check GET request")
	s.addCORSHeaders(c)

	// Check Vision service status
	var message string
	if len(s.vlllmMap) > 0 {
		message = fmt.Sprintf("MCP Vision endpoint is running normally with %d active visual model(s)", len(s.vlllmMap))
	} else {
		message = "MCP Vision endpoint is unhealthy: no active VLLM provider"
	}

	c.String(http.StatusOK, message)
}

// handlePost handles POST image analysis request
// @Summary Image analysis
// @Description Upload image for visual analysis and return result
// @Tags Vision
// @Accept multipart/form-data
// @Produce json
// @Param Authorization header string true "Bearer Token"
// @Param Device-Id header string true "Device ID"
// @Param question formData string true "Question text"
// @Param image formData file true "Image file"
// @Success 200 {object} VisionResponse
// @Failure 400 {object} VisionResponse
// @Failure 401 {object} VisionResponse
// @Failure 500 {object} VisionResponse
// @Router /vision [post]
func (s *DefaultVisionService) handlePost(c *gin.Context) {
	s.addCORSHeaders(c)

	deviceID := c.GetHeader("Device-Id")

	// Verify authentication
	authResult, err := s.verifyAuth(c)
	if err != nil {
		s.respondError(c, http.StatusUnauthorized, err.Error())
		s.logger.Warn("Vision auth failed: %v", err)
		return
	}

	if !authResult.IsValid {
		s.respondError(c, http.StatusUnauthorized, "Invalid auth token or device ID mismatch")
		s.logger.Warn(fmt.Sprintf("Vision auth failed: %s", authResult.DeviceID))
		return
	}

	// Parse multipart form
	req, err := s.parseMultipartRequest(c, deviceID)
	if err != nil {
		s.respondError(c, http.StatusBadRequest, err.Error())
		s.logger.Warn(fmt.Sprintf("Vision request parsing failed: %v", err))
		return
	}

	s.logger.Debug("Received Vision analysis request: %v", map[string]interface{}{
		"device_id":  req.DeviceID,
		"client_id":  req.ClientID,
		"question":   req.Question,
		"image_size": len(req.Image),
		"image_path": req.ImagePath,
	})

	// Process image analysis
	result, err := s.processVisionRequest(req)

	// Return response
	response := VisionResponse{
		Success: true,
		Result:  result,
	}

	if err != nil {
		s.respondError(c, http.StatusInternalServerError, err.Error())
		s.logger.Warn(fmt.Sprintf("Vision request processing failed: %v", err))
		// Return response
		response.Success = false
		response.Message = err.Error()
		response.Result = "" // Clear result
	}

	s.logger.Info("Vision analysis result (success=%t): %s", response.Success, response.Result)
	c.JSON(http.StatusOK, response)
}

// verifyAuth validates authentication token
func (s *DefaultVisionService) verifyAuth(c *gin.Context) (*AuthVerifyResult, error) {
	// Retrieve Authorization header
	authHeader := c.GetHeader("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, fmt.Errorf("invalid or expired authentication token")
	}

	token := authHeader[7:] // Strip "Bearer " prefix

	// Log auth token
	s.logger.Debug(fmt.Sprintf("Received auth token: %s", token))

	// Verify token
	isValid, deviceID, err := s.authToken.VerifyToken(token)
	if err != nil || !isValid {
		s.logger.Warn(fmt.Sprintf("Auth token validation failed: %v", err))
		return nil, fmt.Errorf("invalid or expired authentication token")
	}

	// Check device ID match
	requestDeviceID := c.GetHeader("Device-Id")
	if requestDeviceID != deviceID {
		s.logger.Warn(
			fmt.Sprintf("Device ID does not match token: requestDeviceID=%s, tokenDeviceID=%s", requestDeviceID, deviceID),
		)
		return nil, fmt.Errorf("device ID does not match token")
	}

	return &AuthVerifyResult{
		IsValid:  true,
		DeviceID: deviceID,
	}, nil
}

// parseMultipartRequest parses multipart form request
func (s *DefaultVisionService) parseMultipartRequest(
	c *gin.Context,
	deviceID string,
) (*VisionRequest, error) {
	// Parse multipart form
	err := c.Request.ParseMultipartForm(MAX_FILE_SIZE)
	if err != nil {
		return nil, fmt.Errorf("failed to parse multipart form: %v", err)
	}

	// Log all form fields
	s.logger.Info("Parsed form fields:")
	if c.Request.MultipartForm != nil {
		// Log all text fields
		for key, values := range c.Request.MultipartForm.Value {
			s.logger.Info(fmt.Sprintf("Text field %s: %v", key, values))
		}
		// Log all file fields
		for key, files := range c.Request.MultipartForm.File {
			s.logger.Info(fmt.Sprintf("File field %s: %d files", key, len(files)))
			for i, file := range files {
				s.logger.Info(
					fmt.Sprintf("  File %d: %s (size: %d bytes)", i+1, file.Filename, file.Size),
				)
			}
		}
	}

	// Retrieve question field
	question := c.Request.FormValue("question")
	if question == "" {
		return nil, fmt.Errorf("missing question field")
	}

	// Retrieve image file
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		return nil, fmt.Errorf("missing image file: %v", err)
	}
	defer file.Close()

	// Check file size
	if header.Size > MAX_FILE_SIZE {
		return nil, fmt.Errorf("image size exceeds limit, maximum allowed is %dMB", MAX_FILE_SIZE/1024/1024)
	}

	// Read image bytes
	imageData, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %v", err)
	}

	if len(imageData) == 0 {
		return nil, fmt.Errorf("image data is empty")
	}

	// Validate image format
	if !s.isValidImageFile(imageData) {
		return nil, fmt.Errorf("unsupported file format, please upload a valid image (JPEG, PNG, GIF, BMP, TIFF, WEBP supported)")
	}

	// Save image locally
	saveImageToFile, err := s.saveImageToFile(imageData, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to save image file (%s): %v", saveImageToFile, err)
	}

	return &VisionRequest{
		Question:  question,
		Image:     imageData,
		DeviceID:  deviceID,
		ClientID:  c.GetHeader("Client-Id"),
		ImagePath: saveImageToFile, // Path to saved image
	}, nil
}

func (s *DefaultVisionService) saveImageToFile(imageData []byte, deviceID string) (string, error) {
	// Generate unique filename
	device_id_format := strings.ReplaceAll(deviceID, ":", "_")
	filename := fmt.Sprintf(
		"%s_%d.%s",
		device_id_format,
		time.Now().Unix(),
		s.detectImageFormat(imageData),
	)
	filepath := fmt.Sprintf("uploads/%s", filename)

	// Ensure uploads directory exists
	if err := os.MkdirAll("uploads", os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create uploads directory: %v", err)
	}

	// Save image file
	if err := os.WriteFile(filepath, imageData, 0o644); err != nil {
		return "", fmt.Errorf("failed to save image file: %v", err)
	}

	s.logger.Info(fmt.Sprintf("Image saved to: %s", filepath))
	return filepath, nil
}

// processVisionRequest handles visual analysis request
func (s *DefaultVisionService) processVisionRequest(req *VisionRequest) (string, error) {
	// Select VLLM provider
	provider := s.selectProvider("")
	if provider == nil {
		return "", fmt.Errorf("no visual analysis model available")
	}

	// Convert image to base64
	imageBase64 := base64.StdEncoding.EncodeToString(req.Image)

	// Create ImageData structure
	imageData := image.ImageData{
		Data:   imageBase64,
		Format: s.detectImageFormat(req.Image),
	}
	s.logger.Debug("Processing image data for %s, format: %s", req.ClientID, imageData.Format)
	// Call VLLM provider
	messages := []providers.Message{} // Empty history
	responseChan, err := provider.ResponseWithImage(
		context.Background(),
		"",
		messages,
		imageData,
		req.Question,
	)
	if err != nil {
		return "", fmt.Errorf("failed to call VLLM: %v", err)
	}

	// Collect response chunks
	var result strings.Builder
	for content := range responseChan {
		result.WriteString(content)
	}
	s.logger.Info(fmt.Sprintf("VLLM analysis result: %s", result.String()))

	return result.String(), nil
}

// selectProvider selects VLLM provider
func (s *DefaultVisionService) selectProvider(modelName string) *vlllm.Provider {
	// If model name specified, attempt to match provider
	if modelName != "" {
		if provider, exists := s.vlllmMap[modelName]; exists {
			return provider
		}
	}

	// Otherwise return first available provider
	for _, provider := range s.vlllmMap {
		return provider
	}

	return nil
}

// isValidImageFile checks for valid image header
func (s *DefaultVisionService) isValidImageFile(data []byte) bool {
	if len(data) < 8 {
		return false
	}

	// Check common image format magic bytes
	return s.hasJPEGHeader(data) ||
		s.hasPNGHeader(data) ||
		s.hasGIFHeader(data) ||
		s.hasBMPHeader(data) ||
		s.hasWebPHeader(data)
}

// hasJPEGHeader checks JPEG header
func (s *DefaultVisionService) hasJPEGHeader(data []byte) bool {
	return len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8
}

// hasPNGHeader checks PNG header
func (s *DefaultVisionService) hasPNGHeader(data []byte) bool {
	return len(data) >= 8 &&
		data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 &&
		data[4] == 0x0D && data[5] == 0x0A && data[6] == 0x1A && data[7] == 0x0A
}

// hasGIFHeader checks GIF header
func (s *DefaultVisionService) hasGIFHeader(data []byte) bool {
	return len(data) >= 6 &&
		((data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x38 && data[4] == 0x37 && data[5] == 0x61) ||
			(data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x38 && data[4] == 0x39 && data[5] == 0x61))
}

// hasBMPHeader checks BMP header
func (s *DefaultVisionService) hasBMPHeader(data []byte) bool {
	return len(data) >= 2 && data[0] == 0x42 && data[1] == 0x4D
}

// hasWebPHeader checks WebP header
func (s *DefaultVisionService) hasWebPHeader(data []byte) bool {
	return len(data) >= 12 &&
		data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50
}

// detectImageFormat detects image format
func (s *DefaultVisionService) detectImageFormat(data []byte) string {
	if s.hasJPEGHeader(data) {
		return "jpeg"
	}
	if s.hasPNGHeader(data) {
		return "png"
	}
	if s.hasGIFHeader(data) {
		return "gif"
	}
	if s.hasBMPHeader(data) {
		return "bmp"
	}
	if s.hasWebPHeader(data) {
		return "webp"
	}
	return "jpeg" // Default fallback format
}

// addCORSHeaders adds CORS headers
func (s *DefaultVisionService) addCORSHeaders(c *gin.Context) {
	c.Header("Access-Control-Allow-Headers", "client-id, content-type, device-id, authorization")
	c.Header("Access-Control-Allow-Credentials", "true")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
}

// respondError writes error response
func (s *DefaultVisionService) respondError(c *gin.Context, statusCode int, message string) {
	response := VisionResponse{
		Success: false,
		Message: message,
	}
	c.JSON(statusCode, response)
}

// Cleanup frees resources
func (s *DefaultVisionService) Cleanup() error {
	for name, provider := range s.vlllmMap {
		if err := provider.Cleanup(); err != nil {
			s.logger.Warn(fmt.Sprintf("Failed to clean up VLLM provider %s: %v", name, err))
		}
	}
	s.logger.Info("Vision service cleanup completed")
	return nil
}

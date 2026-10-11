package vision

// VisionRequest represents Vision analysis request parsed from multipart form
// @Description Vision analysis request body
// @Param question formData string true "Question text"
// @Param image formData file true "Image file"
// @Param Device-Id header string true "Device ID"
// @Param Client-Id header string false "Client ID"
// @Param image_path formData string false "Image path"
type VisionRequest struct {
	Question  string // Question text
	Image     []byte // Image data
	DeviceID  string // Device ID from header
	ClientID  string // Client ID from header
	ImagePath string // Image file path
}

// VisionResponse represents standard Vision analysis response
// @Description Vision analysis response body
// @Success 200 {object} VisionResponse
// @Failure 400 {object} VisionResponse
// @Failure 401 {object} VisionResponse
// @Failure 500 {object} VisionResponse
type VisionResponse struct {
	Success bool   `json:"success"`           // Success indicator
	Result  string `json:"result,omitempty"`  // Analysis result
	Message string `json:"message,omitempty"` // Error message
}

// VisionStatusResponse represents Vision health status response
// @Description Vision health status response
type VisionStatusResponse struct {
	Message string // Status message
}

// AuthVerifyResult represents authentication verification result
type AuthVerifyResult struct {
	IsValid  bool
	DeviceID string
}

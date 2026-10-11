package image

// ImageData image data struct
type ImageData struct {
	URL    string `json:"url,omitempty"`    // Image URL
	Data   string `json:"data,omitempty"`   // Base64 encoded image data
	Format string `json:"format,omitempty"` // Image format: jpeg, png, webp, gif
}

// ValidationResult image validation result
type ValidationResult struct {
	IsValid      bool   // Whether valid
	Format       string // Actual format
	Width        int    // Image width
	Height       int    // Image height
	FileSize     int64  // File size
	Error        error  // Error message
	SecurityRisk string // Security risk description
}

// EnhancedMessage enhanced message struct supporting images
type EnhancedMessage struct {
	Type      string    `json:"type"`                 // Message type: text, image, mixed
	Text      string    `json:"text,omitempty"`       // Text content
	ImageData ImageData `json:"image_data,omitempty"` // Image data
}

// ImageMetrics image processing statistics
type ImageMetrics struct {
	TotalProcessed    int64 // Total processed count
	URLDownloads      int64 // URL download count
	Base64Direct      int64 // Base64 direct process count
	FailedValidations int64 // Validation failure count
	SecurityIncidents int64 // Security incident count
}

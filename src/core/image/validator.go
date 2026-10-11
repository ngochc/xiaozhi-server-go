package image

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"strings"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/utils"

	_ "image/gif"  // Register GIF decoder
	_ "image/jpeg" // Register JPEG decoder
	_ "image/png"  // Register PNG decoder

	_ "golang.org/x/image/webp" // Register WEBP decoder
)

// ImageSecurityValidator image security validator
type ImageSecurityValidator struct {
	config *configs.SecurityConfig
	logger *utils.Logger
}

// NewImageSecurityValidator creates new image security validator
func NewImageSecurityValidator(
	config *configs.SecurityConfig,
	logger *utils.Logger,
) *ImageSecurityValidator {
	return &ImageSecurityValidator{
		config: config,
		logger: logger,
	}
}

// Image format magic numbers - JPEG format validation fix
var imageSignatures = map[string][]byte{
	"jpeg": {0xFF, 0xD8}, // JPEG requires first two bytes
	"jpg":  {0xFF, 0xD8}, // Same as JPEG
	"png":  {0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
	"gif":  {0x47, 0x49, 0x46, 0x38},
	"webp": {0x52, 0x49, 0x46, 0x46}, // RIFF, requires WEBP identifier check
	"bmp":  {0x42, 0x4D},
}

// ValidateImageData validates image data
func (v *ImageSecurityValidator) ValidateImageData(imageData ImageData) ValidationResult {
	result := ValidationResult{IsValid: false}

	var imageBytes []byte
	var err error

	// Get image bytes based on data type
	if imageData.Data != "" {
		// Base64 data
		imageBytes, err = base64.StdEncoding.DecodeString(imageData.Data)
		if err != nil {
			result.Error = fmt.Errorf("base64 decode failed: %v", err)
			result.SecurityRisk = "invalid base64 data"
			return result
		}
	} else {
		result.Error = fmt.Errorf("missing image data")
		return result
	}

	// Execute deep validation
	return v.deepValidateImage(imageBytes, imageData.Format)
}

// deepValidateImage performs deep validation of image
func (v *ImageSecurityValidator) deepValidateImage(
	data []byte,
	declaredFormat string,
) ValidationResult {
	result := ValidationResult{IsValid: false}

	// 1. Basic size check
	if int64(len(data)) > v.config.MaxFileSize {
		result.Error = fmt.Errorf(
			"file size exceeded: %d bytes, max allowed: %d bytes",
			len(data),
			v.config.MaxFileSize,
		)
		result.SecurityRisk = "file too large, possible DoS attack"
		v.logger.Warn("detected oversized file", map[string]interface{}{
			"size":     len(data),
			"max_size": v.config.MaxFileSize,
			"format":   declaredFormat,
		})
		return result
	}

	// 2. Format support check
	if declaredFormat != "" && !v.isFormatAllowed(declaredFormat) {
		result.Error = fmt.Errorf("unsupported format: %s", declaredFormat)
		result.SecurityRisk = "disallowed format used"
		return result
	}

	// 3. Malicious content detection
	if v.config.EnableDeepScan && v.scanForMaliciousContent(data) {
		result.Error = fmt.Errorf("detected potentially malicious content")
		result.SecurityRisk = "possible malicious payload"
		v.logger.Warn("detected suspicious content", map[string]interface{}{
			"format": declaredFormat,
			"size":   len(data),
		})
		return result
	}

	// 4. Attempt image decoding for detailed metadata
	decodeResult := v.validateImageDecoding(data, declaredFormat)
	if !decodeResult.IsValid {
		// Image decode failed, check file header match
		if declaredFormat != "" && !v.validateFileSignature(data, declaredFormat) {
			// Log warning without failing immediately
			v.logger.Warn("file header validation failed, attempting decode anyway", map[string]interface{}{
				"declared_format": declaredFormat,
				"actual_header":   fmt.Sprintf("%x", data[:min(len(data), 16)]),
			})
		}
		return decodeResult
	}

	// Image decode succeeded, validation passed
	return decodeResult
}

// validateFileSignature validates file header signature
func (v *ImageSecurityValidator) validateFileSignature(data []byte, format string) bool {
	signature, exists := imageSignatures[strings.ToLower(format)]
	if !exists {
		return false
	}

	if len(data) < len(signature) {
		return false
	}

	// Check if file header matches
	for i, b := range signature {
		if data[i] != b {
			return false
		}
	}

	// WEBP requires additional verification
	if strings.ToLower(format) == "webp" && len(data) >= 12 {
		webpSignature := data[8:12]
		return bytes.Equal(webpSignature, []byte("WEBP"))
	}

	return true
}

// isFormatAllowed checks if format is allowed
func (v *ImageSecurityValidator) isFormatAllowed(format string) bool {
	formatLower := strings.ToLower(format)
	for _, allowedFormat := range v.config.AllowedFormats {
		if strings.ToLower(allowedFormat) == formatLower {
			return true
		}
	}
	return false
}

// scanForMaliciousContent scans for malicious content
func (v *ImageSecurityValidator) scanForMaliciousContent(data []byte) bool {
	// First verify if this is a genuine image file
	// If decodable as image, suspicious byte sequences are likely benign
	reader := bytes.NewReader(data)
	if _, _, err := image.DecodeConfig(reader); err == nil {
		v.logger.Debug("file successfully decoded as image, skipping heuristic malware checks")
		// For decodable images, perform basic security checks only
		return v.basicSecurityCheck(data)
	}

	v.logger.Info("file cannot be decoded as standard image, performing full security check")
	return v.fullSecurityCheck(data)
}

// basicSecurityCheck performs basic security checks on decodable images
func (v *ImageSecurityValidator) basicSecurityCheck(data []byte) bool {
	// Only check obvious threats: executable signatures at start of file
	executableSignatures := [][]byte{
		{0x4D, 0x5A},             // PE header (MZ) - start of file
		{0x7F, 0x45, 0x4C, 0x46}, // ELF header - start of file
	}

	signatureNames := []string{"PE", "ELF"}

	for i, signature := range executableSignatures {
		if bytes.HasPrefix(data, signature) {
			v.logger.Warn("executable signature detected at file start", map[string]interface{}{
				"signature_type": signatureNames[i],
				"signature_hex":  fmt.Sprintf("%x", signature),
			})
			return true
		}
	}

	// Check script content in SVG
	dataStr := string(data)
	if strings.Contains(strings.ToLower(dataStr), "<svg") {
		return v.checkSVGScripts(dataStr)
	}

	v.logger.Debug("basic security check passed")
	return false
}

// fullSecurityCheck performs full security checks on undecodable files
func (v *ImageSecurityValidator) fullSecurityCheck(data []byte) bool {
	// Check executable signatures at file start
	executableSignatures := [][]byte{
		{0x4D, 0x5A},             // PE header (MZ)
		{0x7F, 0x45, 0x4C, 0x46}, // ELF header
		{0xCA, 0xFE, 0xBA, 0xBE}, // Mach-O header
	}

	signatureNames := []string{"PE", "ELF", "Mach-O"}

	for i, signature := range executableSignatures {
		if bytes.HasPrefix(data, signature) {
			v.logger.Warn("executable signature detected at file start", map[string]interface{}{
				"signature_type": signatureNames[i],
				"signature_hex":  fmt.Sprintf("%x", signature),
			})
			return true
		}
	}

	// Check archive signatures (ZIP/GZIP) at file start
	compressionSignatures := [][]byte{
		{0x50, 0x4B, 0x03, 0x04}, // ZIP header
		{0x1F, 0x8B, 0x08},       // GZIP header
	}

	compressionNames := []string{"ZIP", "GZIP"}

	for i, signature := range compressionSignatures {
		if bytes.HasPrefix(data, signature) {
			v.logger.Warn("archive signature detected at file start", map[string]interface{}{
				"signature_type": compressionNames[i],
				"signature_hex":  fmt.Sprintf("%x", signature),
			})
			return true
		}
	}

	// Check script content in SVG
	dataStr := string(data)
	if strings.Contains(strings.ToLower(dataStr), "<svg") {
		return v.checkSVGScripts(dataStr)
	}

	v.logger.Info("full security check passed")
	return false
}

// checkSVGScripts checks script contents in SVG
func (v *ImageSecurityValidator) checkSVGScripts(dataStr string) bool {
	suspiciousStrings := []string{
		"<script",
		"javascript:",
		"vbscript:",
		"onload=",
		"onerror=",
		"eval(",
		"document.cookie",
		"window.location",
		"<iframe",
		"<object",
		"<embed",
	}

	dataStrLower := strings.ToLower(dataStr)
	for _, suspicious := range suspiciousStrings {
		if strings.Contains(dataStrLower, suspicious) {
			v.logger.Warn("suspicious script content detected in SVG", map[string]interface{}{
				"suspicious_content": suspicious,
			})
			return true
		}
	}

	return false
}

// validateImageDecoding validates image decoding
func (v *ImageSecurityValidator) validateImageDecoding(
	data []byte,
	format string,
) ValidationResult {
	result := ValidationResult{Format: format}
	reader := bytes.NewReader(data)

	// Decode validation using standard library
	config, actualFormat, err := image.DecodeConfig(reader)
	if err != nil {
		result.Error = fmt.Errorf("image decode failed: %v", err)
		result.SecurityRisk = "possible malicious payload or corrupted image data"
		return result
	}

	// Update actual format
	if actualFormat != "" {
		result.Format = actualFormat
	}

	// Check dimension limits
	if config.Width > v.config.MaxWidth || config.Height > v.config.MaxHeight {
		result.Error = fmt.Errorf("image dimensions exceeded: %dx%d, max allowed: %dx%d",
			config.Width, config.Height, v.config.MaxWidth, v.config.MaxHeight)
		result.SecurityRisk = "image too large, excessive resource consumption"
		return result
	}

	// Check total pixels
	totalPixels := int64(config.Width) * int64(config.Height)
	if totalPixels > v.config.MaxPixels {
		result.Error = fmt.Errorf("total pixels exceeded: %d, max allowed: %d", totalPixels, v.config.MaxPixels)
		result.SecurityRisk = "excessive pixels, possible memory exhaustion"
		return result
	}

	// Validation succeeded
	result.IsValid = true
	result.Width = config.Width
	result.Height = config.Height
	result.FileSize = int64(len(data))

	v.logger.Debug("image validation succeeded %v", map[string]interface{}{
		"format": result.Format,
		"width":  result.Width,
		"height": result.Height,
		"size":   result.FileSize,
	})

	return result
}

// min helper: returns minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

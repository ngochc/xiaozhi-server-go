package image

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/utils"

	"github.com/google/uuid"
)

// ImageProcessor image processor
type ImageProcessor struct {
	config     *configs.VLLMConfig
	validator  *ImageSecurityValidator
	logger     *utils.Logger
	tempDir    string
	metrics    *ImageMetrics
	httpClient *http.Client
}

// NewImageProcessor creates new image processor
func NewImageProcessor(config *configs.VLLMConfig, logger *utils.Logger) (*ImageProcessor, error) {
	// Create temporary directory
	tempDir := filepath.Join("tmp", "images")
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %v", err)
	}

	// Create security validator
	validator := NewImageSecurityValidator(&config.Security, logger)

	// Configure HTTP client
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Limit redirects to 3
			if len(via) >= 3 {
				return fmt.Errorf("stopped redirect: exceeded max redirect limit")
			}
			return nil
		},
	}

	return &ImageProcessor{
		config:     config,
		validator:  validator,
		logger:     logger,
		tempDir:    tempDir,
		metrics:    &ImageMetrics{},
		httpClient: httpClient,
	}, nil
}

// ProcessImage processes image data and returns base64 string
func (p *ImageProcessor) ProcessImage(ctx context.Context, imageData ImageData) (string, error) {
	atomic.AddInt64(&p.metrics.TotalProcessed, 1)

	var finalImageData ImageData

	// Process image based on input type
	if imageData.URL != "" {
		// Handle URL image
		atomic.AddInt64(&p.metrics.URLDownloads, 1)

		base64Data, err := p.processURLImage(ctx, imageData.URL, imageData.Format)
		if err != nil {
			atomic.AddInt64(&p.metrics.FailedValidations, 1)
			return "", fmt.Errorf("URL image processing failed: %v", err)
		}

		finalImageData = ImageData{
			Data:   base64Data,
			Format: imageData.Format,
		}

		p.logger.Info("URL image processing succeeded", map[string]interface{}{
			"url":    imageData.URL,
			"format": imageData.Format,
		})

	} else if imageData.Data != "" {
		// Handle base64 data directly
		atomic.AddInt64(&p.metrics.Base64Direct, 1)
		finalImageData = imageData

		p.logger.Debug("Base64 image processing started %v", map[string]interface{}{
			"format":      imageData.Format,
			"data_length": len(imageData.Data),
		})
	} else {
		return "", fmt.Errorf("image data is empty: neither URL nor base64 provided")
	}

	// Security validation
	validationResult := p.validator.ValidateImageData(finalImageData)
	if !validationResult.IsValid {
		atomic.AddInt64(&p.metrics.FailedValidations, 1)
		if validationResult.SecurityRisk != "" {
			atomic.AddInt64(&p.metrics.SecurityIncidents, 1)
			p.logger.Warn("security threat detected", map[string]interface{}{
				"error":         validationResult.Error.Error(),
				"security_risk": validationResult.SecurityRisk,
				"format":        finalImageData.Format,
			})
		}
		return "", fmt.Errorf("image validation failed: %v", validationResult.Error)
	}

	p.logger.Debug("image processing completed %v", map[string]interface{}{
		"format":    validationResult.Format,
		"width":     validationResult.Width,
		"height":    validationResult.Height,
		"file_size": validationResult.FileSize,
	})

	return finalImageData.Data, nil
}

// processURLImage processes URL image
func (p *ImageProcessor) processURLImage(
	ctx context.Context,
	url string,
	format string,
) (string, error) {
	// Create unique temporary filename
	tempFileName := fmt.Sprintf("img_%d_%s", time.Now().UnixNano(), uuid.New().String())
	if format != "" {
		tempFileName += "." + format
	}
	tempPath := filepath.Join(p.tempDir, tempFileName)

	// Ensure temporary file is removed on exit
	defer func() {
		if err := os.Remove(tempPath); err != nil && !os.IsNotExist(err) {
			p.logger.Warn("failed to delete temporary file", map[string]interface{}{
				"path":  tempPath,
				"error": err.Error(),
			})
		}
	}()

	// Download image
	if err := p.downloadImage(ctx, url, tempPath); err != nil {
		return "", fmt.Errorf("failed to download image: %v", err)
	}

	// Read file and convert to base64
	imageData, err := os.ReadFile(tempPath)
	if err != nil {
		return "", fmt.Errorf("failed to read temporary file: %v", err)
	}

	// Convert to base64
	base64Data := base64.StdEncoding.EncodeToString(imageData)

	p.logger.Info("URL image download and conversion completed", map[string]interface{}{
		"url":         url,
		"temp_path":   tempPath,
		"file_size":   len(imageData),
		"base64_size": len(base64Data),
	})

	return base64Data, nil
}

// downloadImage downloads image to temporary file
func (p *ImageProcessor) downloadImage(ctx context.Context, url string, tempPath string) error {
	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Set User-Agent to avoid rejection by servers
	req.Header.Set("User-Agent", "XiaoZhi-Image-Bot/1.0")

	// Send request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP response error: %d %s", resp.StatusCode, resp.Status)
	}

	// Check Content-Type
	contentType := resp.Header.Get("Content-Type")
	if !p.isValidImageContentType(contentType) {
		return fmt.Errorf("invalid Content-Type: %s", contentType)
	}

	// Check Content-Length
	if resp.ContentLength > p.config.Security.MaxFileSize {
		return fmt.Errorf("file too large: %d bytes, max allowed: %d bytes",
			resp.ContentLength, p.config.Security.MaxFileSize)
	}

	// Create temporary file
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %v", err)
	}
	defer tempFile.Close()

	// Use LimitReader to prevent unbounded download size
	limitedReader := io.LimitReader(resp.Body, p.config.Security.MaxFileSize)

	// Copy data to temporary file
	written, err := io.Copy(tempFile, limitedReader)
	if err != nil {
		return fmt.Errorf("failed to download file: %v", err)
	}

	p.logger.Info("image download completed", map[string]interface{}{
		"url":          url,
		"content_type": contentType,
		"size":         written,
		"temp_path":    tempPath,
	})

	return nil
}

// isValidImageContentType checks if Content-Type is a valid image type
func (p *ImageProcessor) isValidImageContentType(contentType string) bool {
	validContentTypes := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/gif",
		"image/webp",
		"image/bmp",
	}

	contentTypeLower := strings.ToLower(contentType)
	for _, validType := range validContentTypes {
		if strings.Contains(contentTypeLower, validType) {
			return true
		}
	}

	return false
}

// GetMetrics gets processing metrics
func (p *ImageProcessor) GetMetrics() ImageMetrics {
	return ImageMetrics{
		TotalProcessed:    atomic.LoadInt64(&p.metrics.TotalProcessed),
		URLDownloads:      atomic.LoadInt64(&p.metrics.URLDownloads),
		Base64Direct:      atomic.LoadInt64(&p.metrics.Base64Direct),
		FailedValidations: atomic.LoadInt64(&p.metrics.FailedValidations),
		SecurityIncidents: atomic.LoadInt64(&p.metrics.SecurityIncidents),
	}
}

// Cleanup cleans up resources
func (p *ImageProcessor) Cleanup() error {
	// Clean up old files in temporary directory
	entries, err := os.ReadDir(p.tempDir)
	if err != nil {
		return fmt.Errorf("failed to read temporary directory: %v", err)
	}

	now := time.Now()
	cleanedCount := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filePath := filepath.Join(p.tempDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Delete temporary files older than 1 hour
		if now.Sub(info.ModTime()) > time.Hour {
			if err := os.Remove(filePath); err != nil {
				p.logger.Warn("failed to delete expired temporary file", map[string]interface{}{
					"path":  filePath,
					"error": err.Error(),
				})
			} else {
				cleanedCount++
			}
		}
	}

	if cleanedCount > 0 {
		p.logger.Info("temporary file cleanup completed", map[string]interface{}{
			"cleaned_count": cleanedCount,
		})
	}

	return nil
}

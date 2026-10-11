package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// QuickReplyCache quick reply cache configuration
type QuickReplyCache struct {
	CacheDir    string // Cache directory, default "wake_replay"
	TTSProvider string // TTS provider name
	VoiceName   string // Voice name
	AudioFormat string // Audio format, default "mp3"
}

// NewQuickReplyCache creates quick reply cache configuration
func NewQuickReplyCache(ttsProvider, voiceName string) *QuickReplyCache {
	return &QuickReplyCache{
		CacheDir:    "wake_replay",
		TTSProvider: ttsProvider,
		VoiceName:   voiceName,
		AudioFormat: "mp3",
	}
}

// FindCachedAudio searches for cached quick reply audio file
func (qrc *QuickReplyCache) FindCachedAudio(text string) string {
	// Check if directory exists
	if _, err := os.Stat(qrc.CacheDir); os.IsNotExist(err) {
		return ""
	}

	// Generate filename
	filename := qrc.generateFilename(text)

	// Build full file path
	fullPath := fmt.Sprintf("%s/%s", qrc.CacheDir, filename)

	// Check if file exists
	if _, err := os.Stat(fullPath); err == nil {
		return fullPath
	}

	return ""
}

// SaveCachedAudio saves quick reply audio to cache directory
func (qrc *QuickReplyCache) SaveCachedAudio(text, sourcePath string) error {
	// Create cache directory
	if err := os.MkdirAll(qrc.CacheDir, 0o755); err != nil {
		return fmt.Errorf("failed to create cache directory: %v", err)
	}

	// Generate target filename
	filename := qrc.generateFilename(text)
	targetPath := fmt.Sprintf("%s/%s", qrc.CacheDir, filename)

	// Check if target file already exists
	if _, err := os.Stat(targetPath); err == nil {
		return nil // File already exists, skip saving
	}

	// Copy file to target location
	return qrc.copyFile(sourcePath, targetPath)
}

// generateFilename generates quick reply audio filename
func (qrc *QuickReplyCache) generateFilename(text string) string {
	// Sanitize text
	safeText := qrc.sanitizeFilename(text)

	// Generate filename format: text_provider_voice.format
	filename := fmt.Sprintf(
		"%s_%s_%s.%s",
		safeText,
		qrc.TTSProvider,
		qrc.VoiceName,
		qrc.AudioFormat,
	)

	return filename
}

// sanitizeFilename cleans filename, removing unsafe characters
func (qrc *QuickReplyCache) sanitizeFilename(text string) string {
	// Remove or replace unsafe characters in filename
	unsafe := regexp.MustCompile(`[<>:"/\\|?*\s]+`)
	safe := unsafe.ReplaceAllString(text, "_")

	// Limit filename length to avoid overflow
	if len(safe) > 50 {
		safe = safe[:50]
	}

	// Trim leading and trailing underscores
	safe = strings.Trim(safe, "_")

	// If empty after sanitization, use default name
	if safe == "" {
		safe = "quick_reply"
	}

	return safe
}

// copyFile copies a file
func (qrc *QuickReplyCache) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer targetFile.Close()

	// Copy file contents
	if _, err := targetFile.ReadFrom(sourceFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %v", err)
	}

	return nil
}

// IsQuickReplyHit checks if text is a quick reply phrase
func IsQuickReplyHit(text string, quickReplyWords []string) bool {
	return IsInArray(text, quickReplyWords)
}

// IsCachedFile determines if specified path is a cached file
func (qrc *QuickReplyCache) IsCachedFile(filePath string) bool {
	if filePath == "" {
		return false
	}

	// Get directory portion of file path
	dir := filepath.Dir(filePath)

	// Check if parent directory is the cache directory
	return dir == qrc.CacheDir ||
		strings.HasSuffix(dir, "/"+qrc.CacheDir) ||
		strings.HasSuffix(dir, "\\"+qrc.CacheDir)
}

package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MAX_FILENAME_LENGTH = 250 // Maximum filename length limit
	USER_FILE_SUBSIZE   = 20  // Fixed length subtracted from user filename (date and suffix)
)

// AudioCache audio cache manager
type AudioCache struct {
	CacheDir      string // Cache directory
	TTSProvider   string // TTS provider name
	VoiceName     string // Voice name
	AudioFormat   string // Audio format, default "mp3"
	SampleRate    int    // Sample rate
	Channels      int    // Channels
	BitsPerSample int    // Bits per sample
	DeviceID      string // Device ID to differentiate device caches

	CacheFileSubSize int // Fixed length subtracted from filename
}

// NewAudioCache creates audio cache configuration
func NewAudioCache(ttsProvider, cacheDir, voiceName, audioFormat string) *AudioCache {
	return &AudioCache{
		CacheDir:      cacheDir,
		TTSProvider:   ttsProvider,
		VoiceName:     voiceName,
		AudioFormat:   audioFormat,
		SampleRate:    24000,
		Channels:      1,
		BitsPerSample: 16,

		CacheFileSubSize: len(ttsProvider) + len(voiceName) + 10, // Fixed length subtracted from filename
	}
}

func (ac *AudioCache) SetAudioInfo(sampleRate int, channels int, bitsPerSample int) {
	ac.SampleRate = sampleRate
	ac.Channels = channels
	ac.BitsPerSample = bitsPerSample
}

func (ac *AudioCache) SetDeviceID(deviceID string) {
	ac.DeviceID = strings.Replace(deviceID, ":", "_", -1)
}

// FindCachedAudio searches for cached audio file
func (ac *AudioCache) FindCachedAudio(text string) string {
	// Check if directory exists
	if _, err := os.Stat(ac.CacheDir); os.IsNotExist(err) {
		return ""
	}

	// Generate filename
	filename := ac.generateFilename(text, "mp3")

	// Build full file path
	fullPath := fmt.Sprintf("%s/%s", ac.CacheDir, filename)

	// Check if file exists
	if _, err := os.Stat(fullPath); err == nil {
		return fullPath
	}

	// If not found, replace extension and check wav
	fullPath = strings.Replace(fullPath, ".mp3", ".wav", 1)
	if _, err := os.Stat(fullPath); err == nil {
		return fullPath
	}

	return ""
}

// SaveCachedAudio saves audio to cache directory
func (ac *AudioCache) SaveCachedAudio(text string, data []byte) (string, error) {
	suffix := "wav"
	dir := ac.CacheDir

	// Generate target filename
	filename := ""
	targetPath := ""
	if ac.VoiceName == "user" && ac.DeviceID != "" {
		filename = ac.generateUserFileName(text, suffix)
		dir = fmt.Sprintf("%s/%s", ac.CacheDir, ac.DeviceID)
		targetPath = fmt.Sprintf("%s/%s", dir, filename)
	} else {
		filename = ac.generateFilename(text, suffix)
		targetPath = fmt.Sprintf("%s/%s", ac.CacheDir, filename)
	}

	// Create cache directory
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %v", err)
	}

	// Check if target file already exists
	if _, err := os.Stat(targetPath); err == nil {
		//DefaultLogger.Info("Audio file already exists, skipping save: %s", targetPath)
		return targetPath, nil // File already exists, skip saving
	}

	return SaveAudioToWavFile(data, targetPath, ac.SampleRate, ac.Channels, ac.BitsPerSample, false)
}

func (ac *AudioCache) generateUserFileName(text, suffix string) string {
	// Sanitize text
	safeText := ac.sanitizeFilename(text, USER_FILE_SUBSIZE)
	if suffix == "" {
		suffix = ac.AudioFormat
	}

	// Generate filename format: xxYxxMXXD_text.format
	filename := fmt.Sprintf("%02d%02d%02d%02d%02d%02d_%s.%s",
		time.Now().Year(), time.Now().Month(), time.Now().Day(),
		time.Now().Hour(), time.Now().Minute(), time.Now().Second(),
		safeText, suffix)
	return filename
}

// generateFilename generates audio filename
func (ac *AudioCache) generateFilename(text, suffix string) string {
	// Sanitize text
	safeText := ac.sanitizeFilename(text, ac.CacheFileSubSize)

	if suffix == "" {
		suffix = ac.AudioFormat
	}
	// Generate filename format: text_provider_voice.format
	filename := fmt.Sprintf(
		"%s_%s_%s.%s",
		safeText,
		ac.TTSProvider,
		ac.VoiceName,
		suffix,
	)

	return filename
}

// sanitizeFilename cleans filename, removing unsafe characters
func (ac *AudioCache) sanitizeFilename(text string, l int) string {

	// Remove or replace unsafe characters in filename
	unsafe := regexp.MustCompile(`[\\/:*?"<>|\x00-\x1f]`)
	safe := unsafe.ReplaceAllString(text, "_")

	// Trim leading and trailing underscores, dots, and spaces
	safe = strings.Trim(safe, "_. ")

	maxTextLen := MAX_FILENAME_LENGTH - l // Limit text length to avoid filename overflow
	if len(safe) > maxTextLen {
		safe = safe[:maxTextLen]
		// Check if the last character was truncated
		for len(safe) > 0 {
			r, size := utf8.DecodeLastRuneInString(safe)
			if r == utf8.RuneError && size == 1 {
				safe = safe[:len(safe)-1]
				fmt.Println("Truncating incomplete trailing character, new length:", len(safe), "character:", safe)
			} else {
				break
			}
		}
	}

	// If empty after sanitization, use default name
	if safe == "" {
		safe = "audio"
	}

	return safe
}

// IsAudioCacheHit checks if text is an audio cache hit
func IsAudioCacheHit(text string, audioCacheWords []string) bool {
	return IsInArray(text, audioCacheWords)
}

// IsCachedFile determines if specified path is a cached file
func (ac *AudioCache) IsCachedFile(filePath string) bool {
	if filePath == "" {
		return false
	}

	// Get directory portion of file path
	dir := filepath.Dir(filePath)

	// Check if parent directory is the cache directory
	return dir == ac.CacheDir ||
		strings.HasSuffix(dir, "/"+ac.CacheDir) ||
		strings.HasSuffix(dir, "\\"+ac.CacheDir)
}

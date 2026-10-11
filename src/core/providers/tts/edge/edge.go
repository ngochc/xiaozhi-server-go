package edge

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
	"xiaozhi-server-go/src/core/providers/tts"

	"github.com/wujunwei928/edge-tts-go/edge_tts"
)

// Provider Edge TTS provider implementation
type Provider struct {
	*tts.BaseProvider
}

// NewProvider creates Edge TTS provider
func NewProvider(config *tts.Config, deleteFile bool) (*Provider, error) {
	base := tts.NewBaseProvider(config, deleteFile)
	return &Provider{
		BaseProvider: base,
	}, nil
}

// ToTTS converts text to audio file and returns file path
// Uses github.com/wujunwei928/edge-tts-go with 24k sample rate
func (p *Provider) ToTTS(text string) (string, error) {
	// Get configured voice or use default if unset
	edgeTTSStartTime := time.Now()
	voice := p.BaseProvider.Config().Voice
	if voice == "" {
		voice = "zh-CN-XiaoxiaoNeural" // Default voice
	}

	// Create temporary file path for edgeTTS MP3
	outputDir := p.BaseProvider.Config().OutputDir
	if outputDir == "" {
		outputDir = os.TempDir() // Use system temp dir if not configured
	}
	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory '%s': %v", outputDir, err)
	}
	// Use a unique filename
	tempFile := filepath.Join(outputDir, fmt.Sprintf("edge_tts_go_%d.mp3", time.Now().UnixNano()))

	// Configure edge-tts-go options
	connOptions := []edge_tts.CommunicateOption{
		edge_tts.SetVoice(voice),
	}

	// Create Communicate instance
	conn, err := edge_tts.NewCommunicate(text, connOptions...)
	if err != nil {
		return "", fmt.Errorf("failed to create edge-tts-go Communicate: %v", err)
	}

	// Get audio stream data
	audioData, err := conn.Stream()
	if err != nil {
		return "", fmt.Errorf("edge-tts-go failed to get audio stream: %v", err)
	}

	ttsDuration := time.Since(edgeTTSStartTime)
	_ = ttsDuration
	//fmt.Println(fmt.Sprintf("edge-tts-go speech synthesis complete, elapsed: %s", ttsDuration))

	// Write audio data to temporary file
	err = os.WriteFile(tempFile, audioData, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write audio file '%s': %v", tempFile, err)
	}

	// Check if file created successfully
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		return "", fmt.Errorf("edge-tts-go failed to create audio file: %s", tempFile)
	}
	//fmt.Printf("audio file generated: %s\n", tempFile)

	// Return the path to the generated audio file
	return tempFile, nil
}

func init() {
	// Register Edge TTS provider
	tts.Register("edge", func(config *tts.Config, deleteFile bool) (tts.Provider, error) {
		return NewProvider(config, deleteFile)
	})
}

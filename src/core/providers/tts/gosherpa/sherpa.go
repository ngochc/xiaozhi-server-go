package gosherpa

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"xiaozhi-server-go/src/core/providers/tts"

	"github.com/gorilla/websocket"
)

// Provider Sherpa TTS provider implementation
type Provider struct {
	*tts.BaseProvider
	conn *websocket.Conn
}

// NewProvider creates Sherpa TTS provider
func NewProvider(config *tts.Config, deleteFile bool) (*Provider, error) {
	base := tts.NewBaseProvider(config, deleteFile)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second, // Set handshake timeout
	}
	conn, _, err := dialer.DialContext(context.Background(), config.Cluster, map[string][]string{})
	if err != nil {
		return nil, err
	}

	return &Provider{
		BaseProvider: base,
		conn:         conn,
	}, nil
}

// ToTTS converts text to audio file and returns file path
func (p *Provider) ToTTS(text string) (string, error) {
	// Get configured voice or use default if unset
	SherpaTTSStartTime := time.Now()

	// Create temporary file path for SherpaTTS MP3
	outputDir := p.BaseProvider.Config().OutputDir
	if outputDir == "" {
		outputDir = os.TempDir() // Use system temp dir if not configured
	}
	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory '%s': %v", outputDir, err)
	}
	// Use a unique filename
	tempFile := filepath.Join(outputDir, fmt.Sprintf("go_sherpa_tts_%d.wav", time.Now().UnixNano()))

	p.conn.WriteMessage(websocket.TextMessage, []byte(text))
	_, bytes, err := p.conn.ReadMessage()

	if err != nil {
		return "", fmt.Errorf("go-sherpa-tts failed to get audio stream: %v", err)
	}

	ttsDuration := time.Since(SherpaTTSStartTime)
	fmt.Println(fmt.Sprintf("go-sherpa-tts speech synthesis complete, elapsed: %s", ttsDuration))

	// Write audio data to temporary file
	err = os.WriteFile(tempFile, bytes, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write audio file '%s': %v", tempFile, err)
	}

	// Check if file created successfully
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		return "", fmt.Errorf("go-sherpa-tts failed to create audio file: %s", tempFile)
	}

	// Return the path to the generated audio file
	return tempFile, nil
}

func init() {
	// Register Sherpa TTS provider
	tts.Register("gosherpa", func(config *tts.Config, deleteFile bool) (tts.Provider, error) {
		return NewProvider(config, deleteFile)
	})
}

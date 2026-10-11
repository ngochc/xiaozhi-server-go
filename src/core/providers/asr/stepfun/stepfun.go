package stepfun

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"xiaozhi-server-go/src/core/providers/asr"
	"xiaozhi-server-go/src/core/utils"

	"encoding/json"
	"github.com/gorilla/websocket"
)

// Ensure Provider implements asr.Provider interface
var _ asr.Provider = (*Provider)(nil)

// Provider StepFun ASR provider implementation
type Provider struct {
	*asr.BaseProvider

	// Configuration
	apiKey string
	model  string
	voice  string
	wsURL  string
	logger *utils.Logger
	prompt string

	// Streaming recognition fields
	conn        *websocket.Conn
	isStreaming bool
	result      string
	err         error
	connMutex   sync.Mutex
}

func NewProvider(config *asr.Config, deleteFile bool, logger *utils.Logger) (*Provider, error) {
	base := asr.NewBaseProvider(config, deleteFile)

	// Retrieve configuration from config.Data
	apiKey, ok := config.Data["api_key"].(string)
	if !ok {
		return nil, fmt.Errorf("missing api_key configuration")
	}

	model, ok := config.Data["model"].(string)
	if !ok {
		return nil, fmt.Errorf("missing model configuration")
	}
	voice, ok := config.Data["voice"].(string)
	if !ok {
		return nil, fmt.Errorf("missing voice configuration")
	}

	prompt, ok := config.Data["prompt"].(string)
	if !ok {
		prompt = "You are an AI assistant capable of multi-language conversations."
	}

	provider := &Provider{
		BaseProvider: base,
		apiKey:       apiKey,
		model:        model,
		voice:        voice,
		wsURL:        fmt.Sprintf("wss://api.stepfun.com/v1/realtime?model=%s", model),
		logger:       logger,
		prompt:       prompt,
	}

	provider.InitAudioProcessing()

	return provider, nil
}

// Ensure Provider implements asr.Provider interface
var _ asr.Provider = (*Provider)(nil)

// AddAudio adds audio data to buffer
func (p *Provider) AddAudio(data []byte) error {
	return p.AddAudioWithContext(context.Background(), data)
}

// AddAudioWithContext adds audio data with context
func (p *Provider) AddAudioWithContext(ctx context.Context, data []byte) error {
	// Check and start streaming recognition
	p.connMutex.Lock()
	isStreaming := p.isStreaming
	p.connMutex.Unlock()

	if !isStreaming {
		if err := p.StartStreaming(ctx); err != nil {
			return err
		}
	}

	// Send audio data (Base64 encoded)
	if len(data) > 0 {
		if err := p.sendAppendAudio(data); err != nil {
			return err
		}
	}

	return nil
}

// SendLastAudio submits audio buffer to finish recognition round immediately
func (p *Provider) SendLastAudio(data []byte) error {
	if len(data) > 0 {
		if err := p.AddAudio(data); err != nil {
			return err
		}
	}
	return p.sendJSON(map[string]interface{}{
		"event_id": fmt.Sprintf("event_%d", time.Now().UnixNano()),
		"type":     "input_audio_buffer.commit",
	})
}

// Transcribe recognizes audio segment directly
func (p *Provider) Transcribe(ctx context.Context, audioData []byte) (string, error) {
	if p.isStreaming {
		return "", fmt.Errorf("streaming recognition in progress, call Reset first")
	}
	if err := p.StartStreaming(ctx); err != nil {
		return "", err
	}
	defer p.Cleanup()

	if err := p.AddAudioWithContext(ctx, audioData); err != nil {
		return "", err
	}
	// Non-blocking return via listener callback
	return p.result, nil
}

// StartStreaming connects to Step Realtime WebSocket and sends session.update
func (p *Provider) StartStreaming(ctx context.Context) error {
	p.logger.Info("----Starting Step streaming recognition----")
	p.ResetStartListenTime()

	if p.isStreaming {
		p.logger.Debug("Step streaming recognition already started, skipping init")
		return nil
	}

	// Mutex protects connection initialization
	p.connMutex.Lock()
	// Ensure old connection is closed
	if p.conn != nil {
		p.logger.Debug("Step streaming recognition closing old connection")
		p.closeConnection()
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))

	conn, resp, err := dialer.DialContext(ctx, p.wsURL, headers)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		return fmt.Errorf("WebSocket connection failed (status code: %d): %v", status, err)
	}
	p.logger.Debug("Step streaming recognition established WebSocket successfully")
	p.conn = conn
	p.connMutex.Unlock()

	// Send session.update
	sessionPayload := map[string]interface{}{
		"event_id": fmt.Sprintf("event_%d", time.Now().UnixNano()),
		"type":     "session.update",
		"session": map[string]interface{}{
			"modalities":          []string{"text", "audio"},
			"instructions":        p.prompt,
			"voice":               p.voice,
			"input_audio_format":  "pcm16",
			"output_audio_format": "pcm16",
			"turn_detection": map[string]interface{}{
				"type":                       "server_vad",
				"energy_awakeness_threshold": 100, // Activation threshold
			},
		},
	}
	if err := p.sendJSON(sessionPayload); err != nil {
		p.connMutex.Lock()
		_ = p.conn.Close()
		p.conn = nil
		p.connMutex.Unlock()
		return fmt.Errorf("failed to send session.update: %v", err)
	}

	// Flag streaming and start reader goroutine
	p.isStreaming = true
	go p.readLoop()
	return nil
}

func (p *Provider) sendAppendAudio(data []byte) error {
	// Encode PCM16 bytes to Base64
	encoded := base64.StdEncoding.EncodeToString(data)
	payload := map[string]interface{}{
		"event_id": fmt.Sprintf("event_%d", time.Now().UnixNano()),
		"type":     "input_audio_buffer.append",
		"audio":    encoded,
	}

	return p.sendJSON(payload)
}

func (p *Provider) sendJSON(v interface{}) error {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	if p.conn == nil {
		return fmt.Errorf("WebSocket connection does not exist")
	}
	bytes, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return p.conn.WriteMessage(websocket.TextMessage, bytes)
}

func (p *Provider) readLoop() {
	p.logger.Info("Step streaming recognition goroutine started")
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("Step streaming recognition goroutine panic: %v", r)
		}
		p.connMutex.Lock()
		p.isStreaming = false
		if p.conn != nil {
			p.closeConnection()
		}
		p.connMutex.Unlock()
		p.logger.Info("Step streaming recognition goroutine ended")
	}()

	var baseEvent BaseEvent
	for {
		if !p.isStreaming || p.conn == nil {
			return
		}
		conn := p.conn

		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			p.setErrorAndStop(err)
			return
		}
		if msgType != websocket.TextMessage {
			// Ignore non-text frames
			continue
		}

		if err := json.Unmarshal(data, &baseEvent); err != nil {
			p.setErrorAndStop(fmt.Errorf("failed to parse server event: %v", err))
			return
		}

		p.logger.Debug("Step streaming recognition event type: %s", baseEvent.Type)
		switch baseEvent.Type {
		case "error":
			e := ErrorEvent{}
			if err := json.Unmarshal(data, &e); err != nil {
				p.logger.Error("failed to parse server event: %v", err)
				return
			}
			p.setErrorAndStop(fmt.Errorf("server error: %v", e.Error.Message))
			return
		case "session.created":
			e := SessionCreatedEvent{}
			if err := json.Unmarshal(data, &e); err != nil {
				p.logger.Error("failed to parse server event: %v", err)
				return
			}
			p.logger.Info("type: %s, sessionID: %s", e.Type, e.Session.ID)
		case "session.updated", "input_audio_buffer.speech_started", "input_audio_buffer.speech_stopped", "input_audio_buffer.committed", "input_audio_buffer.cleared":
			// No special handling needed
			continue
		case "conversation.item.input_audio_transcription.completed":
			e := ConversationItemInputAudioTranscriptionCompletedEvent{}
			if err := json.Unmarshal(data, &e); err != nil {
				p.logger.Error("failed to parse server event: %v", err)
				return
			}
			// Read transcription result
			text := e.Transcript
			p.logger.Debug("[DEBUG] Step recognition result: %s", text)
			p.connMutex.Lock()
			p.result = text
			p.connMutex.Unlock()

			if listener := p.BaseProvider.GetListener(); listener != nil {
				if text == "" && p.SilenceTime() > 30*time.Second {
					p.BaseProvider.SilenceCount += 1
					text = "You did not hear me clearly"
				} else if text != "" {
					p.BaseProvider.SilenceCount = 0
				}
				if finished := listener.OnAsrResult(text, true); finished {
					return
				}
			}
		default:
			// Other events ignored
		}
	}
}

func (p *Provider) setErrorAndStop(err error) {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()
	p.err = err
	p.isStreaming = false
	msg := err.Error()
	if strings.Contains(msg, "use of closed network connection") {
		p.logger.Debug("Step setErrorAndStop: %v", err)
	} else {
		p.logger.Error("Step setErrorAndStop: %v", err)
	}
	if p.conn != nil {
		p.closeConnection()
	}
}

func (p *Provider) closeConnection() {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("error closing connection: %v", r)
		}
	}()
	if p.conn != nil {
		_ = p.conn.Close()
		p.conn = nil
	}
}

// Reset resets ASR state
func (p *Provider) Reset() error {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	p.isStreaming = false
	p.closeConnection()
	p.result = ""
	p.err = nil

	// Reset audio processing
	p.InitAudioProcessing()
	p.logger.Info("Step ASR state reset")
	return nil
}

// Initialize initializes
func (p *Provider) Initialize() error { return nil }

// Cleanup cleans up resources
func (p *Provider) Cleanup() error {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()
	p.closeConnection()
	p.logger.Info("Step ASR resources cleaned up")
	return nil
}

func init() {
	// Register StepFun ASR provider
	asr.Register("stepfun", func(config *asr.Config, deleteFile bool, logger *utils.Logger) (asr.Provider, error) {
		return NewProvider(config, deleteFile, logger)
	})
}

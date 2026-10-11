package doubao

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"xiaozhi-server-go/src/core/providers/asr"
	"xiaozhi-server-go/src/core/utils"

	"github.com/gorilla/websocket"
)

// Protocol constants
const (
	clientFullRequest   = 0x1
	clientAudioRequest  = 0x2
	serverFullResponse  = 0x9
	serverAck           = 0xB
	serverErrorResponse = 0xF
)

// Sequence types
const (
	noSequence  = 0x0
	negSequence = 0x2
)

// Serialization methods
const (
	noSerialization   = 0x0
	jsonFormat        = 0x1
	thriftFormat      = 0x3
	gzipCompression   = 0x1
	customCompression = 0xF

	// Timeout settings
	idleTimeout = 30 * time.Second // End recognition if no new data
)

// Ensure Provider implements asr.Provider interface
var _ asr.Provider = (*Provider)(nil)

// Provider Doubao ASR provider implementation
type Provider struct {
	*asr.BaseProvider
	appID         string
	accessToken   string
	outputDir     string
	host          string
	wsURL         string
	chunkDuration int
	connectID     string
	logger        *utils.Logger // Logger

	// Configuration
	modelName     string
	endWindowSize int
	enablePunc    bool
	enableITN     bool
	enableDDC     bool

	// Streaming recognition fields
	conn        *websocket.Conn
	isStreaming bool
	reqID       string
	result      string
	err         error
	connMutex   sync.Mutex // Mutex protecting connection state

	sendDataCnt int // Counter tracking audio packets sent
}

// NewProvider creates Doubao ASR provider instance
func NewProvider(config *asr.Config, deleteFile bool, logger *utils.Logger) (*Provider, error) {
	base := asr.NewBaseProvider(config, deleteFile)

	// Retrieve config from config.Data
	appID, ok := config.Data["appid"].(string)
	if !ok {
		return nil, fmt.Errorf("missing appid configuration")
	}

	accessToken, ok := config.Data["access_token"].(string)
	if !ok {
		return nil, fmt.Errorf("missing access_token configuration")
	}

	// Ensure output directory exists
	outputDir, _ := config.Data["output_dir"].(string)
	if outputDir == "" {
		outputDir = "tmp/"
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %v", err)
	}

	// Create connection ID
	connectID := fmt.Sprintf("%d", time.Now().UnixNano())
	// Read end_window_size from config, default 400
	endWindowSize := 400 // Default
	if configEndWindowSize, ok := config.Data["end_window_size"]; ok {
		switch v := configEndWindowSize.(type) {
		case float64:
			endWindowSize = int(v)
		case int:
			endWindowSize = v
		case int64:
			endWindowSize = int(v)
		}
	}

	url := "wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_nostream"
	provider := &Provider{
		BaseProvider:  base,
		appID:         appID,
		accessToken:   accessToken,
		outputDir:     outputDir,
		host:          "openspeech.bytedance.com",
		wsURL:         url,
		chunkDuration: 200, // Fixed 200ms chunks
		connectID:     connectID,
		logger:        logger, // Logger

		// Default configuration
		modelName:     "bigmodel",
		endWindowSize: endWindowSize,
		enablePunc:    true,
		enableITN:     true,
		enableDDC:     false,
	}

	// Initialize audio processing
	provider.InitAudioProcessing()

	return provider, nil
}

// Transcribe implements asr.Provider interface
func (p *Provider) Transcribe(ctx context.Context, audioData []byte) (string, error) {
	if p.isStreaming {
		return "", fmt.Errorf("streaming recognition in progress, call Reset first")
	}

	// Create temporary file
	tempFile := filepath.Join(p.outputDir, fmt.Sprintf("temp_%d.wav", time.Now().UnixNano()))
	if err := os.WriteFile(tempFile, audioData, 0o644); err != nil {
		return "", fmt.Errorf("failed to save temporary file: %v", err)
	}
	defer func() {
		if p.DeleteFile() {
			os.Remove(tempFile)
		}
	}()

	// Initialize connection
	if err := p.Initialize(); err != nil {
		return "", err
	}
	defer p.Cleanup()

	// Add audio data
	if err := p.AddAudioWithContext(ctx, audioData); err != nil {
		return "", err
	}
	// Wait for result via callback
	return p.result, nil
}

// generateHeader generates protocol header
func (p *Provider) generateHeader(
	messageType uint8,
	flags uint8,
	serializationMethod uint8,
) []byte {
	header := make([]byte, 4)
	header[0] = (1 << 4) | 1                                 // Protocol version (4 bits) + header size (4 bits)
	header[1] = (messageType << 4) | flags                   // Message type (4 bits) + flags (4 bits)
	header[2] = (serializationMethod << 4) | gzipCompression // Serialization (4 bits) + compression (4 bits)
	header[3] = 0                                            // Reserved
	return header
}

// constructRequest constructs request data
func (p *Provider) constructRequest() map[string]interface{} {
	return map[string]interface{}{
		"user": map[string]interface{}{
			"uid": p.reqID,
		},
		"audio": map[string]interface{}{
			"format": "pcm",
			//"codec":    "opus", // Default raw audio format
			"rate":     16000,
			"bits":     16,
			"channel":  1,
			"language": "zh-CN", // Added language as per doc example
		},
		"request": map[string]interface{}{
			"model_name":      p.modelName,
			"end_window_size": p.endWindowSize,
			"enable_punc":     p.enablePunc,
			"enable_itn":      p.enableITN,
			"enable_ddc":      p.enableDDC,
			"result_type":     "single",
			"show_utterances": false, // Added show_utterances, default to false
		},
	}
}

// GetAudioBuffer gets base audioBuffer
func (p *Provider) GetAudioBuffer() *bytes.Buffer {
	return p.BaseProvider.GetAudioBuffer()
}

type AsrResponsePayload struct {
	AudioInfo struct {
		Duration int `json:"duration"`
	} `json:"audio_info"`
	Result struct {
		Text       string `json:"text"`
		Utterances []struct {
			Definite  bool   `json:"definite"`
			EndTime   int    `json:"end_time"`
			StartTime int    `json:"start_time"`
			Text      string `json:"text"`
			Words     []struct {
				EndTime   int    `json:"end_time"`
				StartTime int    `json:"start_time"`
				Text      string `json:"text"`
			} `json:"words"`
		} `json:"utterances,omitempty"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

type AsrResponse struct {
	Code            int                 `json:"code"`
	Event           int                 `json:"event"`
	IsLastPackage   bool                `json:"is_last_package"`
	PayloadSequence int32               `json:"payload_sequence"`
	PayloadSize     int                 `json:"payload_size"`
	PayloadMsg      *AsrResponsePayload `json:"payload_msg"`
}

// parseResponse parses response data
func (p *Provider) parseResponse(data []byte) (map[string]interface{}, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("response data too short")
	}
	headerSize := data[0] & 0x0f
	messageType := data[1] >> 4
	messageTypeSpecificFlags := data[1] & 0x0f // flags
	serializationMethod := data[2] >> 4
	compressionMethod := data[2] & 0x0f
	var asr_result AsrResponse
	// Parse header
	_ = data[0] >> 4 // protocol version

	var payload []byte
	// Skip header to get payload
	if len(data) > 8 && data[8] == '{' {
		payload = data[8:]
		// p.logger.Info("[DEBUG] payload offset adjusted to data[8:], first byte=%d", payload[0])
	} else {
		payload = data[headerSize*4:]
	}
	result := make(map[string]interface{})

	if messageTypeSpecificFlags&0x01 != 0 {
		asr_result.PayloadSequence = int32(binary.BigEndian.Uint32(payload[:4]))
	}
	if messageTypeSpecificFlags&0x02 != 0 {
		asr_result.IsLastPackage = true
		result["is_last_package"] = true
		//p.logger.Info("received last packet, PayloadSequence=%d", asr_result.PayloadSequence)
	}
	if messageTypeSpecificFlags&0x04 != 0 {
		asr_result.Event = int(binary.BigEndian.Uint32(payload[:4]))
	}

	var payloadMsg []byte
	var payloadSize int32

	switch messageType {
	case serverFullResponse:
		// If payload is JSON directly (starts with '{'), parse directly without sequence/payloadSize
		if len(payload) > 0 && payload[0] == '{' {
			// p.logger.Info("[DEBUG] entering direct JSON parsing branch, payload len=%d", len(payload))
			payloadMsg = payload
			payloadSize = int32(len(payload))
		} else {
			// p.logger.Info("[DEBUG] entering header parsing branch, payload len=%d", len(payload))
			// Doc: Header | Sequence | Payload size | Payload
			if len(payload) < 8 {
				return nil, fmt.Errorf("serverFullResponse payload too short for sequence and size: got %d bytes", len(payload))
			}
			seq := binary.BigEndian.Uint32(payload[0:4])
			result["seq"] = seq // Store WebSocket frame sequence
			payloadSize = int32(binary.BigEndian.Uint32(payload[4:8]))
			if len(payload) < 8+int(payloadSize) {
				return nil, fmt.Errorf("serverFullResponse payload too short for declared payload size: got %d bytes, expected header + %d bytes", len(payload), payloadSize)
			}
			payloadMsg = payload[8:]
		}
	case serverAck:
		// Doc for serverAck is not detailed for ASR, but generally it might have a sequence
		if len(payload) < 4 {
			return nil, fmt.Errorf(
				"serverAck payload too short for sequence: got %d bytes",
				len(payload),
			)
		}
		seq := binary.BigEndian.Uint32(payload[0:4])
		result["seq"] = seq
		if len(payload) >= 8 { // If there's more data, assume it's payload size and then payload
			payloadSize = int32(binary.BigEndian.Uint32(payload[4:8]))
			if len(payload) < 8+int(payloadSize) {
				return nil, fmt.Errorf(
					"serverAck payload too short for declared payload size: got %d bytes, expected header + %d bytes",
					len(payload),
					payloadSize,
				)
			}
			payloadMsg = payload[8:]
		} else {
			// serverAck might not have a payload body, only sequence
			payloadSize = 0
			payloadMsg = nil
		}
	case serverErrorResponse:
		code := uint32(binary.BigEndian.Uint32(payload[:4]))
		result["code"] = code
		payloadSize = int32(binary.BigEndian.Uint32(payload[4:8]))
		payloadMsg = payload[8:]
	}
	asr_result.PayloadSize = int(payloadSize)
	if payloadMsg != nil {
		if compressionMethod == gzipCompression {
			reader, err := gzip.NewReader(bytes.NewReader(payloadMsg))
			if err != nil {
				return nil, fmt.Errorf("failed to decompress response data: %v", err)
			}
			defer reader.Close()

			buf := new(bytes.Buffer)
			if _, err := buf.ReadFrom(reader); err != nil {
				return nil, fmt.Errorf("failed to read decompressed data: %v", err)
			}
			payloadMsg = buf.Bytes()
		}

		if serializationMethod == jsonFormat {
			var jsonData map[string]interface{}
			if err := json.Unmarshal(payloadMsg, &jsonData); err != nil {
				return nil, fmt.Errorf("failed to parse JSON response: %v", err)
			}
			p.logger.Debug("[DEBUG] parseResponse: JSON parsed successfully, data=%v", jsonData)
			result["payload_msg"] = jsonData
		} else if serializationMethod != noSerialization {
			result["payload_msg"] = string(payloadMsg)
		}
	}

	result["payload_size"] = payloadSize
	return result, nil
}

// AddAudio adds audio data to buffer
func (p *Provider) AddAudio(data []byte) error {
	return p.AddAudioWithContext(context.Background(), data)
}

// AddAudioWithContext adds audio data with context
func (p *Provider) AddAudioWithContext(ctx context.Context, data []byte) error {
	// Use mutex to check state
	p.connMutex.Lock()
	isStreaming := p.isStreaming
	p.connMutex.Unlock()

	if !isStreaming {
		err := p.StartStreaming(ctx)
		if err != nil {
			return err
		}
	}

	// Check if actual data needs sending
	if len(data) > 0 && p.isStreaming {
		// Send audio data directly
		if err := p.sendAudioData(data, false); err != nil {
			return err
		} else {
			p.sendDataCnt += 1
			if p.sendDataCnt%20 == 0 {
				p.logger.Debug("audio data sent successfully, length: %d bytes", len(data))
			}
		}
	}

	return nil
}

func (p *Provider) StartStreaming(ctx context.Context) error {
	p.logger.Info("[ASR] [doubao] [streaming recognition] started")
	p.ResetStartListenTime()
	// Mutex protects connection initialization
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	// Double check to avoid concurrent initialization
	if p.isStreaming {
		return nil
	}

	// Initialize streaming recognition
	p.InitAudioProcessing()
	p.result = ""
	p.err = nil

	// Ensure old connection is closed
	if p.conn != nil {
		p.closeConnection()
	}

	// Establish WebSocket connection
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second, // Set handshake timeout
	}
	headers := map[string][]string{
		"X-Api-App-Key":     {p.appID},
		"X-Api-Access-Key":  {p.accessToken},
		"X-Api-Resource-Id": {"volc.bigasr.sauc.duration"},
		"X-Api-Connect-Id":  {p.connectID},
	}

	// Retry mechanism
	var conn *websocket.Conn
	var resp *http.Response
	var err error
	maxRetries := 2

	for i := 0; i <= maxRetries; i++ {
		conn, resp, err = dialer.DialContext(ctx, p.wsURL, headers)
		if err == nil {
			break
		}

		if i < maxRetries {
			backoffTime := time.Duration(500*(i+1)) * time.Millisecond
			fmt.Printf("WebSocket connection failed (attempt %d/%d): %v, retrying in %v\n",
				i+1, maxRetries+1, err, backoffTime)
			time.Sleep(backoffTime)
		}
	}

	if err != nil {
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}
		return fmt.Errorf("WebSocket connection failed (status code: %d): %v", statusCode, err)
	}

	p.conn = conn

	// Send initial request
	p.reqID = fmt.Sprintf("%d", time.Now().UnixNano())
	request := p.constructRequest()
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to construct request data: %v", err)
	}

	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	if _, err := gzipWriter.Write(requestBytes); err != nil {
		return fmt.Errorf("failed to compress request data: %v", err)
	}
	gzipWriter.Close()

	compressedRequest := buf.Bytes()
	header := p.generateHeader(clientFullRequest, noSequence, jsonFormat)

	// Construct full request
	size := make([]byte, 4)
	binary.BigEndian.PutUint32(size, uint32(len(compressedRequest)))
	fullRequest := append(header, size...)
	fullRequest = append(fullRequest, compressedRequest...)

	// Send request
	if err := p.conn.WriteMessage(websocket.BinaryMessage, fullRequest); err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}

	// Read response
	_, response, err := p.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	} else {
		p.logger.Debug("[DEBUG] streaming recognition: received WebSocket message length=%d", len(response))
	}

	initialResult, err := p.parseResponse(response)
	if err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	// Check initial response status
	if msg, ok := initialResult["payload_msg"].(map[string]interface{}); ok {
		// Doubao ASR v3 uses 20000000 for success code in initial response
		if code, ok := msg["code"].(float64); ok && int(code) != 20000000 {
			return fmt.Errorf("ASR initialization error: %v", msg)
		}
	}

	p.isStreaming = true
	p.logger.Debug("[DEBUG] streaming recognition initialized successfully, connectID=%s, reqID=%s", p.connectID, p.reqID)
	// Launch goroutine to process streaming response
	go func() {
		p.ReadMessage()
	}()
	return nil
}

func (p *Provider) ReadMessage() {
	p.logger.Info("[ASR] [doubao] streaming recognition goroutine started")
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("streaming recognition goroutine error: %v", r)
		}
		p.connMutex.Lock()
		p.isStreaming = false // Mark streaming recognition finished
		if p.conn != nil {
			p.closeConnection()
		}
		p.connMutex.Unlock()
		p.logger.Info("[ASR] [doubao] streaming recognition goroutine ended")
	}()

	for {
		// Check connection status to prevent read after close
		p.connMutex.Lock()
		if !p.isStreaming || p.conn == nil {
			p.connMutex.Unlock()
			p.logger.Info("streaming recognition finished or connection closed, exiting read loop")
			return
		}
		conn := p.conn
		p.connMutex.Unlock()

		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		_, response, err := conn.ReadMessage()
		if err != nil {
			p.setErrorAndStop(err)
			return
		}

		result, err := p.parseResponse(response)
		if err != nil {
			p.setErrorAndStop(fmt.Errorf("failed to parse response: %v", err))
			return
		}

		if code, hasCode := result["code"]; hasCode {
			p.logger.Info("detected code field: parsed result=%v", result)
			codeValue := code.(uint32)
			if codeValue != 0 {
				p.setErrorAndStop(fmt.Errorf("ASR server error: Code=%d", codeValue))
				return
			}
		}

		// Handle normal response
		if payloadMsg, ok := result["payload_msg"].(map[string]interface{}); ok {
			// Check for result field
			if resultData, hasResult := payloadMsg["result"].(map[string]interface{}); hasResult {
				// Extract text result
				text := ""
				if textData, hasText := resultData["text"].(string); hasText {
					text = textData
				}

				p.logger.Debug("[DEBUG] streaming recognition: success, text='%s'", text)

				p.connMutex.Lock()
				p.result = text
				p.connMutex.Unlock()
				isLastPackage := false
				if isLast, hasLast := result["is_last_package"]; hasLast && isLast.(bool) {
					// End streaming recognition if last packet
					isLastPackage = true
					p.logger.Info("detected last ASR audio packet, is_last_package=%v", isLast)
				}

				if listener := p.BaseProvider.GetListener(); listener != nil {
					if text == "" && p.SilenceTime() > idleTimeout {
						p.BaseProvider.SilenceCount += 1
						text = "[SILENCE_TIMEOUT] The user has not spoken for a while. Please politely remind the user."
						p.logger.Info("silence timeout detected, SilenceTime=%v/%v", p.SilenceTime(), idleTimeout)
						p.ResetStartListenTime()
					} else if text != "" {
						p.BaseProvider.SilenceCount = 0 // Reset silence count
					}
					if text == "" && !isLastPackage {
						continue
					}
					if finished := listener.OnAsrResult(text, isLastPackage); finished {
						return
					}
				}
			} else if errorData, hasError := payloadMsg["error"]; hasError {
				// Handle error field in response
				p.setErrorAndStop(fmt.Errorf("ASR response error: %v", errorData))
				return
			}
		}

	}
}

func (p *Provider) setErrorAndStop(err error) {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()
	p.err = err
	p.isStreaming = false
	errMsg := err.Error()
	if strings.Contains(errMsg, "use of closed network connection") {
		p.logger.Debug("setErrorAndStop: %v, sendDataCnt=%d", err, p.sendDataCnt)
	} else {
		p.logger.Error("setErrorAndStop: %v, sendDataCnt=%d", err, p.sendDataCnt)
	}

	if p.conn != nil {
		p.closeConnection()
	}
}

func (p *Provider) closeConnection() {
	defer func() {
		if r := recover(); r != nil {
			// Recover panic quietly to avoid crash
			p.logger.Error("error closing connection: %v", r)
		}
	}()

	if p.conn != nil {
		// Close connection directly without close message
		_ = p.conn.Close()
		p.conn = nil
	}
}

func (p *Provider) SendLastAudio(data []byte) error {
	return p.sendAudioData(data, true)
}

// sendAudioData sends audio data directly
func (p *Provider) sendAudioData(data []byte, isLast bool) error {
	p.logger.Debug(
		"[DEBUG] sendAudioData: data length=%d, isLast=%t, sendDataCnt=%d",
		len(data),
		isLast,
		p.sendDataCnt,
	)
	// Do not send if no data and not last frame
	if len(data) == 0 && !isLast {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			// Recover WebSocket write panic to avoid crash
			p.logger.Error("panic sending audio data: %v", r)
		}
	}()

	// Reset closes and clears connection on barge-in; send and reset share mutex,
	// preventing nil pointer write after check.
	p.connMutex.Lock()
	defer p.connMutex.Unlock()
	conn := p.conn
	if conn == nil {
		return fmt.Errorf("WebSocket connection does not exist")
	}

	var compressBuffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressBuffer)
	if _, err := gzipWriter.Write(data); err != nil {
		return fmt.Errorf("failed to compress audio data: %v", err)
	}
	gzipWriter.Close()

	compressedAudio := compressBuffer.Bytes()
	flags := uint8(0)
	if isLast {
		flags = negSequence
	}

	header := p.generateHeader(clientAudioRequest, flags, noSerialization)
	size := make([]byte, 4)
	binary.BigEndian.PutUint32(size, uint32(len(compressedAudio)))

	audioMessage := append(header, size...)
	audioMessage = append(audioMessage, compressedAudio...)

	if err := conn.WriteMessage(websocket.BinaryMessage, audioMessage); err != nil {
		return fmt.Errorf("failed to send audio data: %v", err)
	}

	return nil
}

// Reset resets ASR state
func (p *Provider) Reset() error {
	// Mutex protects state change
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	p.isStreaming = false
	p.closeConnection()

	p.reqID = ""
	p.result = ""
	p.err = nil

	// Reset audio processing
	p.InitAudioProcessing()

	p.logger.Info("[ASR] [State] reset")

	return nil
}

// Initialize implements Provider interface
func (p *Provider) Initialize() error {
	// Ensure output directory exists
	if err := os.MkdirAll(p.outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to initialize output directory: %v", err)
	}
	return nil
}

// Cleanup implements Provider interface
func (p *Provider) Cleanup() error {
	// Mutex protects state change
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	// Ensure WebSocket connection is closed
	p.closeConnection()

	p.logger.Info("ASR resources cleaned up")

	return nil
}

func (p *Provider) CloseConnection() error {
	return nil
}

func init() {
	// Register Doubao ASR provider
	asr.Register("doubao", func(config *asr.Config, deleteFile bool, logger *utils.Logger) (asr.Provider, error) {
		return NewProvider(config, deleteFile, logger)
	})
}

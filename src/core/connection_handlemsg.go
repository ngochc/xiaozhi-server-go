package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"xiaozhi-server-go/src/core/chat"
	"xiaozhi-server-go/src/core/image"
	"xiaozhi-server-go/src/core/providers"
	"xiaozhi-server-go/src/core/utils"
)

// handleMessage processes received messages
func (h *ConnectionHandler) handleMessage(messageType int, message []byte) error {
	switch messageType {
	case 1: // Text message
		h.clientTextQueue <- string(message)
		return nil
	case 2: // Binary message (audio data)
		if h.clientAudioFormat == "pcm" {
			// Enqueue PCM data directly
			h.clientAudioQueue <- message
		} else if h.clientAudioFormat == "opus" {
			// Check if Opus decoder is initialized
			if h.opusDecoder != nil {
				// Decode Opus data to PCM
				decodedData, err := h.opusDecoder.Decode(message)
				if err != nil {
					h.logger.Error(fmt.Sprintf("failed to decode Opus audio: %v", err))
					// Pass raw data to ASR even if decoding fails
					h.clientAudioQueue <- message
				} else {
					// Decode successful, enqueue PCM data
					h.logger.Debug(fmt.Sprintf("Opus decode success: %d bytes -> %d bytes", len(message), len(decodedData)))
					if len(decodedData) > 0 {
						h.clientAudioQueue <- decodedData
					}
				}
			} else {
				// Pass raw data directly if no decoder
				h.clientAudioQueue <- message
			}
		}
		return nil
	default:
		h.logger.Error(fmt.Sprintf("unknown message type: %d", messageType))
		return fmt.Errorf("unknown message type: %d", messageType)
	}
}

// processClientTextMessage handles text data
func (h *ConnectionHandler) processClientTextMessage(ctx context.Context, text string) error {
	// Parse JSON message
	var msgJSON interface{}
	if err := json.Unmarshal([]byte(text), &msgJSON); err != nil {
		return h.conn.WriteMessage(1, []byte(text))
	}

	// Check if integer type
	if _, ok := msgJSON.(float64); ok {
		return h.conn.WriteMessage(1, []byte(text))
	}

	// Parse map type to process specific message
	msgMap, ok := msgJSON.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid message format")
	}

	// Dispatch by message type
	msgType, ok := msgMap["type"].(string)
	if !ok {
		return fmt.Errorf("invalid message type")
	}

	switch msgType {
	case "hello":
		return h.handleHelloMessage(msgMap)
	case "abort":
		return h.clientAbortChat()
	case "listen":
		return h.handleListenMessage(msgMap)
	case "iot":
		return h.handleIotMessage(msgMap)
	case "chat":
		return h.handleChatMessage(ctx, text)
	case "vision":
		return h.handleVisionMessage(msgMap)
	case "image":
		return h.handleImageMessage(ctx, msgMap)
	case "mcp":
		return h.mcpManager.HandleXiaoZhiMCPMessage(msgMap)
	default:
		h.logger.Warn("=== unknown message type ===", map[string]interface{}{
			"unknown_type": msgType,
			"full_message": msgMap,
		})
		return fmt.Errorf("unknown message type: %s", msgType)
	}
}

func (h *ConnectionHandler) handleVisionMessage(msgMap map[string]interface{}) error {
	// Handle visual message
	cmd := msgMap["cmd"].(string)
	if cmd == "gen_pic" {
	} else if cmd == "gen_video" {
	} else if cmd == "read_img" {
	}
	return nil
}

// handleHelloMessage handles hello handshake message
// Client uploads audio format and sample rate
func (h *ConnectionHandler) handleHelloMessage(msgMap map[string]interface{}) error {
	h.LogInfo(fmt.Sprintf("[Client] [hello handshake received] %v", msgMap))
	// Get client encoding format
	if audioParams, ok := msgMap["audio_params"].(map[string]interface{}); ok {
		if format, ok := audioParams["format"].(string); ok {
			h.clientAudioFormat = format
			if format == "pcm" {
				// Client uses PCM format, server also uses PCM format
				h.serverAudioFormat = "pcm"
			}
		}
		if sampleRate, ok := audioParams["sample_rate"].(float64); ok {
			h.clientAudioSampleRate = int(sampleRate)
		}
		if channels, ok := audioParams["channels"].(float64); ok {
			h.clientAudioChannels = int(channels)
		}
		if frameDuration, ok := audioParams["frame_duration"].(float64); ok {
			h.clientAudioFrameDuration = int(frameDuration)
		}
		h.LogInfo(fmt.Sprintf("[Client] [audio parameters %s/%d/%d/%d]",
			h.clientAudioFormat, h.clientAudioSampleRate, h.clientAudioChannels, h.clientAudioFrameDuration))
	}
	h.sendHelloMessage()
	h.closeOpusDecoder()
	// Initialize Opus decoder
	opusDecoder, err := utils.NewOpusDecoder(&utils.OpusDecoderConfig{
		SampleRate:  h.clientAudioSampleRate, // Client sample rate
		MaxChannels: h.clientAudioChannels,   // Mono channel
	})
	if err != nil {
		h.logger.Error(fmt.Sprintf("failed to initialize Opus decoder: %v", err))
	} else {
		h.opusDecoder = opusDecoder
		h.LogInfo("[Opus] [decoder] initialized successfully")
	}

	return nil
}

// handleListenMessage handles speech-related message
func (h *ConnectionHandler) handleListenMessage(msgMap map[string]interface{}) error {

	// Handle state parameter
	state, ok := msgMap["state"].(string)
	if !ok {
		return fmt.Errorf("listen message missing state parameter")
	}

	// Handle mode parameter
	if mode, ok := msgMap["mode"].(string); ok {
		h.clientListenMode = mode
		h.LogInfo(fmt.Sprintf("[Client] [listen mode %s/%s]", h.clientListenMode, state))
		h.providers.asr.SetListener(h)
	}

	switch state {
	case "start":
		if h.client_asr_text != "" && h.clientListenMode == "manual" {
			h.clientAbortChat()
		}
		h.client_asr_text = ""
	case "stop":
		h.providers.asr.SendLastAudio([]byte{}) // Send empty data to mark end
		h.LogInfo("client stopped speech recognition")
	case "detect":
		text, hasText := msgMap["text"].(string)

		if hasText && text != "" {
			// Text only, handle with standard LLM
			h.LogInfo(fmt.Sprintf("[Detect] [plain text %s] handling with LLM", text))
			return h.handleChatMessage(context.Background(), text)
		} else {
			// Neither image nor text
			h.logger.Warn("detect message has neither text nor image parameters")
			return fmt.Errorf("detect message missing text or image parameter")
		}
	}
	return nil
}

// handleIotMessage handles IoT device message
func (h *ConnectionHandler) handleIotMessage(msgMap map[string]interface{}) error {
	if descriptors, ok := msgMap["descriptors"].([]interface{}); ok {
		// Handle device descriptors
		// Process IoT device descriptors
		h.LogInfo(fmt.Sprintf("received IoT device descriptors: %v", descriptors))
	}
	if states, ok := msgMap["states"].([]interface{}); ok {
		// Handle device state
		// Process IoT device states
		h.LogInfo(fmt.Sprintf("received IoT device states: %v", states))
	}
	return nil
}

// handleImageMessage handles image message
func (h *ConnectionHandler) handleImageMessage(ctx context.Context, msgMap map[string]interface{}) error {
	// Increment dialogue round
	h.talkRound++
	currentRound := h.talkRound
	h.LogInfo(fmt.Sprintf("starting new image dialogue round: %d", currentRound))

	// Check for VLLLM Provider
	if h.providers.vlllm == nil {
		h.logger.Warn("VLLLM service not configured, image message will be ignored")
		return h.conn.WriteMessage(1, []byte("Image processing is not currently supported"))
	}

	// Parse text content
	text, ok := msgMap["text"].(string)
	if !ok {
		text = "Please describe this image" // Default prompt
	}

	// Parse image data
	imageDataMap, ok := msgMap["image_data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing image data")
	}

	imageData := image.ImageData{}
	if url, ok := imageDataMap["url"].(string); ok {
		imageData.URL = url
	}
	if data, ok := imageDataMap["data"].(string); ok {
		imageData.Data = data
	}
	if format, ok := imageDataMap["format"].(string); ok {
		imageData.Format = format
	}

	// Validate image data
	if imageData.URL == "" && imageData.Data == "" {
		return fmt.Errorf("image data is empty")
	}

	h.LogInfo(fmt.Sprintf("received image message %v", map[string]interface{}{
		"text":        text,
		"has_url":     imageData.URL != "",
		"has_data":    imageData.Data != "",
		"format":      imageData.Format,
		"data_length": len(imageData.Data),
	}))

	// Send STT message immediately
	err := h.sendSTTMessage(text)
	if err != nil {
		h.logger.Error(fmt.Sprintf("failed to send STT message: %v", err))
		return fmt.Errorf("failed to send STT message: %v", err)
	}

	// Send TTS start state
	if err := h.sendTTSMessage("start", "", 0); err != nil {
		h.logger.Error(fmt.Sprintf("failed to send TTS start state: %v", err))
		return fmt.Errorf("failed to send TTS start state: %v", err)
	}

	// Send thinking emotion
	if err := h.sendEmotionMessage("thinking"); err != nil {
		h.logger.Error(fmt.Sprintf("failed to send thinking emotion message: %v", err))
		return fmt.Errorf("failed to send emotion message: %v", err)
	}

	// Add user message to dialogue history with image description
	userMessage := fmt.Sprintf("%s [User sent a %s image]", text, imageData.Format)
	h.dialogueManager.Put(chat.Message{
		Role:    "user",
		Content: userMessage,
	})

	// Get dialogue history
	messages := make([]providers.Message, 0)
	for _, msg := range h.dialogueManager.GetLLMDialogue() {
		// Exclude message with image description to process via VLLLM
		if msg.Role == "user" && (strings.Contains(msg.Content, "[User sent a") || strings.Contains(msg.Content, "[User sent a")) {
			continue
		}
		messages = append(messages, providers.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return h.genResponseByVLLM(ctx, messages, imageData, text, currentRound)
}

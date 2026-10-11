package core

import (
	"encoding/json"
	"fmt"
	pathutil "path/filepath"
	"sync/atomic"
	"time"
	"xiaozhi-server-go/src/core/utils"
)

// sendHelloMessage sends hello handshake message
func (h *ConnectionHandler) sendHelloMessage() error {
	// Security check
	if h.conn == nil {
		return fmt.Errorf("connection uninitialized, cannot send hello message")
	}

	// Nil checks
	if h.config == nil {
		return fmt.Errorf("config object uninitialized")
	}

	hello := make(map[string]interface{})
	hello["type"] = "hello"
	hello["version"] = 1
	hello["transport"] = "websocket"
	hello["session_id"] = h.sessionID
	hello["audio_params"] = map[string]interface{}{
		"format":         h.serverAudioFormat,
		"sample_rate":    h.serverAudioSampleRate,
		"channels":       h.serverAudioChannels,
		"frame_duration": h.serverAudioFrameDuration,
	}
	data, err := json.Marshal(hello)
	if err != nil {
		return fmt.Errorf("failed to serialize hello message: %v", err)
	}

	return h.conn.WriteMessage(1, data)
}

func (h *ConnectionHandler) sendTTSMessage(state string, text string, textIndex int) error {
	// Send TTS state finish notification
	stateMsg := map[string]interface{}{
		"type":        "tts",
		"state":       state,
		"session_id":  h.sessionID,
		"text":        text,
		"index":       textIndex,
		"audio_codec": "opus", // Using Opus encoding
	}
	data, err := json.Marshal(stateMsg)
	if err != nil {
		return fmt.Errorf("failed to serialize %s state: %v", state, err)
	}
	if err := h.conn.WriteMessage(1, data); err != nil {
		return fmt.Errorf("failed to send %s state: %v", state, err)
	}
	return nil
}

func (h *ConnectionHandler) sendSTTMessage(text string) error {
	sttMsg := map[string]interface{}{
		"type":       "stt",
		"text":       text,
		"session_id": h.sessionID,
	}
	jsonData, err := json.Marshal(sttMsg)
	if err != nil {
		return fmt.Errorf("failed to serialize STT message: %v", err)
	}
	if err := h.conn.WriteMessage(1, jsonData); err != nil {
		return fmt.Errorf("failed to send STT message: %v", err)
	}

	return nil
}

// sendEmotionMessage sends emotion message
func (h *ConnectionHandler) sendEmotionMessage(emotion string) error {
	data := map[string]interface{}{
		"type":       "llm",
		"text":       utils.GetEmotionEmoji(emotion),
		"emotion":    emotion,
		"session_id": h.sessionID,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to serialize emotion message: %v", err)
	}
	return h.conn.WriteMessage(1, jsonData)
}

func (h *ConnectionHandler) sendAudioMessage(filepath string, text string, textIndex int, round int) {
	startTime := time.Now() // Record task start time
	defer func() {
		// Delete audio file after sending if configured
		h.deleteAudioFileIfNeeded(filepath, "audio send completed")

		spentTime := time.Since(startTime).Milliseconds()
		h.LogDebug(fmt.Sprintf("[TTS] [send task %d/%dms/%dms] %s", textIndex, h.tts_last_text_index, spentTime, text))
		h.providers.asr.ResetStartListenTime()
		if textIndex == h.tts_last_text_index {
			if round != h.talkRound {
				h.LogInfo("sendTTSMessage stop: skipping end state, round has changed")
			} else {
				h.sendTTSMessage("stop", "", textIndex)
				if h.closeAfterChat {
					h.Close()
				} else {
					h.clearSpeakStatus()
				}
			}
		}
	}()

	if len(filepath) == 0 {
		return
	}
	// Check round
	if round != h.talkRound {
		h.LogInfo(fmt.Sprintf("sendAudioMessage: skipping expired audio: task round=%d, current round=%d, text=%s",
			round, h.talkRound, text))
		// Delete audio file even if skipped if configured
		h.deleteAudioFileIfNeeded(filepath, "skipping expired round")
		return
	}

	if atomic.LoadInt32(&h.serverVoiceStop) == 1 { // Server voice stopped
		h.LogInfo(fmt.Sprintf("sendAudioMessage: server voice stopped, not sending audio data: %s", text))
		// Delete audio file on server voice stop if configured
		h.deleteAudioFileIfNeeded(filepath, "server voice stopped")
		return
	}

	var audioData [][]byte
	var duration float64
	var err error

	// Convert audio to Opus format using TTS provider
	if h.serverAudioFormat == "pcm" {
		h.LogInfo("server audio format is PCM, sending directly")
		audioData, duration, err = utils.AudioToPCMData(filepath)
		if err != nil {
			h.LogError(fmt.Sprintf("audio conversion to PCM failed: %v", err))
			return
		}
	} else if h.serverAudioFormat == "opus" {
		audioData, duration, err = utils.AudioToOpusData(filepath)
		if err != nil {
			h.LogError(fmt.Sprintf("audio conversion to Opus failed: %v", err))
			return
		}
		if h.config.SaveTTSAudio {
			capturePath, captureErr := utils.SaveOpusPackets(
				pathutil.Join(pathutil.Dir(filepath), "opus_packets"),
				h.deviceID,
				h.sessionID,
				h.serverAudioSampleRate,
				h.serverAudioChannels,
				h.serverAudioFrameDuration,
				audioData,
			)
			if captureErr != nil {
				h.LogError(fmt.Sprintf("failed to save downlink Opus capture: %v", captureErr))
			} else {
				h.LogInfo(fmt.Sprintf("[TTS] saved downlink Opus capture: %s", capturePath))
			}
		}
	}

	// Send TTS state start notification
	if err := h.sendTTSMessage("sentence_start", text, textIndex); err != nil {
		h.LogError(fmt.Sprintf("failed to send TTS start state: %v", err))
		return
	}

	if textIndex == 1 {
		now := time.Now()
		spentTime := now.Sub(h.roundStartTime)
		h.logger.Debug("First sentence latency %s, text: %q, round: %d", spentTime, text, round)
	}
	h.logger.Debug("TTS send (%s): %q (index:%d/%d, duration:%f, frames:%d)", h.serverAudioFormat, text, textIndex, h.tts_last_text_index, duration, len(audioData))

	// Send audio frames with pacing
	if err := h.sendAudioFrames(audioData, text, round); err != nil {
		h.LogError(fmt.Sprintf("paced audio frame sending failed: %v", err))
		return
	}

	// Send TTS state finish notification
	if err := h.sendTTSMessage("sentence_end", text, textIndex); err != nil {
		h.LogError(fmt.Sprintf("failed to send TTS finish state: %v", err))
		return
	}
}

// sendAudioFrames sends paced audio frames to avoid overflowing client buffer
func (h *ConnectionHandler) sendAudioFrames(audioData [][]byte, text string, round int) error {
	if len(audioData) == 0 {
		return nil
	}

	startTime := time.Now()
	playPosition := 0 // Playback position (ms)

	// Pre-buffer: send first few frames to improve playback smoothness
	preBufferFrames := 3
	if len(audioData) < preBufferFrames {
		preBufferFrames = len(audioData)
	}
	preBufferTime := time.Duration(h.serverAudioFrameDuration*preBufferFrames) * time.Millisecond // Pre-buffer duration in ms

	// Send pre-buffer frames
	for i := 0; i < preBufferFrames; i++ {
		// Check if interrupted
		if atomic.LoadInt32(&h.serverVoiceStop) == 1 || round != h.talkRound {
			h.LogInfo(fmt.Sprintf("audio sending interrupted (pre-buffer phase): frame=%d/%d, text=%s", i+1, preBufferFrames, text))
			return nil
		}

		if err := h.conn.WriteMessage(2, audioData[i]); err != nil {
			return fmt.Errorf("failed to send pre-buffer audio frame: %v", err)
		}
		playPosition += h.serverAudioFrameDuration
	}

	// Send remaining audio frames
	remainingFrames := audioData[preBufferFrames:]
	for i, chunk := range remainingFrames {
		// Check if interrupted or round changed
		if atomic.LoadInt32(&h.serverVoiceStop) == 1 || round != h.talkRound {
			h.LogInfo(fmt.Sprintf("audio sending interrupted: frame=%d/%d, text=%s", i+preBufferFrames+1, len(audioData), text))
			return nil
		}

		// Check if connection closed
		select {
		case <-h.stopChan:
			return nil
		default:
		}

		// Calculate expected send time
		expectedTime := startTime.Add(time.Duration(playPosition)*time.Millisecond - preBufferTime)
		currentTime := time.Now()
		delay := expectedTime.Sub(currentTime)

		// Flow control pacing
		if delay > 0 {
			// Interruptible sleep
			ticker := time.NewTicker(10 * time.Millisecond) // 10ms check interval
			defer ticker.Stop()

			endTime := time.Now().Add(delay)
			for time.Now().Before(endTime) {
				select {
				case <-ticker.C:
					// Check interrupt conditions
					if atomic.LoadInt32(&h.serverVoiceStop) == 1 || round != h.talkRound {
						h.LogInfo(fmt.Sprintf("audio sending interrupted during pacing delay: frame=%d/%d, text=%s", i+preBufferFrames+1, len(audioData), text))
						return nil
					}
				case <-h.stopChan:
					return nil
				}
			}
		}

		// Send audio frame
		if err := h.conn.WriteMessage(2, chunk); err != nil {
			return fmt.Errorf("failed to send audio frame: %v", err)
		}

		playPosition += h.serverAudioFrameDuration
	}
	time.Sleep(preBufferTime) // Ensure pre-buffer time elapsed
	spentTime := time.Since(startTime).Milliseconds()
	h.LogInfo(fmt.Sprintf("[TTS] [audio frames %d/%dms/%dms] %s", len(audioData), playPosition, spentTime, text))
	return nil
}

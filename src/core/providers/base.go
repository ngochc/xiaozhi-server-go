package providers

import (
	"context"
	"xiaozhi-server-go/src/core/types"
)

// Provider base interface for all providers
type Provider interface {
	Initialize() error
	Cleanup() error
}

type AsrEventListener interface {
	OnAsrResult(result string, isFinalResult bool) bool
}

// ASRProvider speech recognition provider interface
type ASRProvider interface {
	Provider
	// Recognize audio data directly
	Transcribe(ctx context.Context, audioData []byte) (string, error)
	// Add audio data to buffer
	AddAudio(data []byte) error

	// Send final audio chunk and mark as complete
	SendLastAudio(data []byte) error

	SetListener(listener AsrEventListener)

	// Set user preferences such as language
	SetUserPreferences(preferences map[string]interface{}) error

	// Reset ASR state
	Reset() error

	// Disconnect persistent ASR connection
	CloseConnection() error

	// Get current silence count
	GetSilenceCount() int

	ResetSilenceCount()

	ResetStartListenTime()

	EnableSilenceDetection(bEnable bool)
}

// TTSProvider text-to-speech provider interface
type TTSProvider interface {
	Provider

	// Synthesize speech and return audio file path
	ToTTS(text string) (string, error)

	SetVoice(voice string) (error, string)
}

// LLMProvider Large Language Model provider interface
type LLMProvider interface {
	types.LLMProvider
}

// Message dialogue message
type Message = types.Message

package types

type AudioChunk struct {
	Text      string // Associated text content
	Index     int    // Text index marking sequence within current round
	Round     int    // Conversation round index
	Data      []byte // Audio data
	Timestamp int64  // Timestamp marking chunk sequence
	EOF       bool   // Whether this is the final audio chunk
	Encoding  string // Audio encoding format, e.g. "mp3", "wav"
}

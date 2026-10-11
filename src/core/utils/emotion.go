package utils

import "regexp"

// EmotionEmoji defines mapping from emotions to emojis
var EmotionEmoji = map[string]string{
	"neutral":     "😐",
	"happy":       "😊",
	"laughing":    "😂",
	"funny":       "🤡",
	"sad":         "😢",
	"angry":       "😠",
	"crying":      "😭",
	"loving":      "🥰",
	"embarrassed": "😳",
	"surprised":   "😮",
	"shocked":     "😱",
	"thinking":    "🤔",
	"winking":     "😉",
	"cool":        "😎",
	"relaxed":     "😌",
	"delicious":   "😋",
	"kissy":       "😘",
	"confident":   "😏",
	"sleepy":      "😴",
	"silly":       "🤪",
	"confused":    "😕",
}

// GetEmotionEmoji returns corresponding emoji based on emotion
func GetEmotionEmoji(emotion string) string {
	if emoji, ok := EmotionEmoji[emotion]; ok {
		return emoji
	}
	return EmotionEmoji["neutral"] // Default to neutral emoji
}

// Simplified emoji regular expression
var SimpleEmojiRegex = regexp.MustCompile(`[\x{1F000}-\x{1FFFF}]|` +
	`[\x{2600}-\x{26FF}]|` + // Miscellaneous symbols
	`[\x{2700}-\x{27BF}]`) // Dingbats

func RemoveAllEmoji(text string) string {
	return SimpleEmojiRegex.ReplaceAllString(text, "")
}

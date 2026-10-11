package utils

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	// Precompiled regular expressions
	reSplitString          = regexp.MustCompile(`[.,!?;。！？；：]+`)
	reMarkdownChars        = regexp.MustCompile(`(\*\*|__|\*|_|#{1,6}\s|` + "`" + `{1,3}|~~|>\s|\[.*?\]\(.*?\)|\!\[.*?\]\(.*?\)|\|.*?\|)`)
	reRemoveAllPunctuation = regexp.MustCompile(
		`[.,!?;:，。！？、；：""''「」『』（）\(\)【】\[\]{}《》〈〉—–\-_~·…‖\|\\/*&\^%\$#@\+=<>]`,
	)
	reWakeUpWord             = regexp.MustCompile(`^(?i)(hello|hi|hey|\x{4f60}\x{597d}).+`)
	reRemoveParenthesesCN    = regexp.MustCompile(`（[^）]*）`)  // Chinese parentheses
	reRemoveParenthesesEN    = regexp.MustCompile(`\([^)]*\)`)  // English parentheses
)

// SplitAtLastPunctuation splits text at the last punctuation mark, optimizing sentence segmentation for chat
func SplitAtLastPunctuation(text string) (string, int) {
	if len(text) == 0 {
		return "", 0
	}

	// Define sentence-splitting punctuations by priority
	// Priority 1: Strong pause punctuations (period, question mark, exclamation mark, etc.)
	strongPunctuations := []string{"。", "？", "！", "；", "?", "!", ";"}

	// Priority 2: Medium pause punctuations (comma, colon, etc.)
	mediumPunctuations := []string{"，", "：", ",", ".", ":"}

	// Priority 3: Light pause punctuations (enumeration comma, brackets, etc.)
	lightPunctuations := []string{"、", "）", ")", "】", "]", "》", ">", "`", "'"}

	// Dynamically adjust min segment length to prevent exceeding text length
	minLength := 2
	if len(text) < minLength {
		minLength = 1
	}

	// Prioritize strong pause punctuation
	if segment, pos := findLastPunctuationWithMinLength(text, strongPunctuations, minLength); pos > 0 {
		return segment, pos
	}

	// If text is long (>50 chars), consider medium pause punctuation
	if len(text) > 50 {
		minLength = 8
		if len(text) < minLength {
			minLength = len(text) / 2
		}
		if segment, pos := findLastPunctuationWithMinLength(text, mediumPunctuations, minLength); pos > 0 {
			return segment, pos
		}
	}

	// If text is very long (>80 chars), consider light pause punctuation
	if len(text) > 80 {
		minLength = 8
		if len(text) < minLength {
			minLength = len(text) / 2
		}
		if segment, pos := findLastPunctuationWithMinLength(text, lightPunctuations, minLength); pos > 0 {
			return segment, pos
		}
	}

	// If no suitable punctuation found and text is long (>100 chars), force split at space
	if len(text) > 100 {
		minLength = 8
		if len(text) < minLength {
			minLength = len(text) / 2
		}
		if segment, pos := findLastSpaceWithMinLength(text, minLength); pos > 0 {
			return segment, pos
		}
	}

	// If text is overly long (>120 chars), force split
	if len(text) > 120 {
		cutPos := 80
		if len(text) < cutPos {
			cutPos = len(text) / 2
		}
		return text[:cutPos], cutPos
	}

	return "", 0
}

// findLastPunctuationWithMinLength finds last punctuation position ensuring minLength
func findLastPunctuationWithMinLength(text string, punctuations []string, minLength int) (string, int) {
	// Safety check: ensure minLength does not exceed text length
	if minLength >= len(text) {
		minLength = len(text) - 1
		if minLength < 0 {
			return "", 0
		}
	}

	lastIndex := -1
	foundPunctuation := ""

	for _, punct := range punctuations {
		// Search starting from minLength
		searchText := text[minLength:]
		if idx := strings.LastIndex(searchText, punct); idx != -1 {
			actualIdx := idx + minLength

			// If period between digits, treat as decimal point and skip
			if punct == "." {
				// Ensure indices within bounds before checking (rune-safe decode)
				if actualIdx > 0 && actualIdx < len(text) {
					// Decode preceding rune
					beforeRune, _ := utf8.DecodeLastRuneInString(text[:actualIdx])
					// Decode following rune
					afterStart := actualIdx + len(punct)
					if afterStart < len(text) {
						afterRune, _ := utf8.DecodeRuneInString(text[afterStart:])
						// Only treat as decimal point when digits on both sides
						if beforeRune >= '0' && beforeRune <= '9' && afterRune >= '0' && afterRune <= '9' {
							continue
						}
					}
				}
			}

			if actualIdx > lastIndex {
				lastIndex = actualIdx
				foundPunctuation = punct
			}
		}
	}

	if lastIndex == -1 {
		return "", 0
	}

	endPos := lastIndex + len(foundPunctuation)

	// Check if following closing quotes or brackets should be preserved
	endPos = adjustForClosingQuotes(text, endPos)

	// Ensure does not exceed text length
	if endPos > len(text) {
		endPos = len(text)
	}
	return text[:endPos], endPos
}

// adjustForClosingQuotes adjusts end position to preserve closing quotes and brackets
func adjustForClosingQuotes(text string, pos int) int {
	if pos >= len(text) {
		return pos
	}

	// Convert to rune slice to handle UTF-8 correctly
	runes := []rune(text)

	// Convert byte index pos to rune index
	runePos := len([]rune(text[:pos]))

	// Handle Chinese closing quotes
	chineseClosingQuotes := map[rune]bool{
		'”': true, // Chinese closing double quote
		'’': true, // Chinese closing single quote
	}

	// Look ahead up to 2 characters (adjacent quotes only)
	maxLookAhead := 2

	for i := 0; i < maxLookAhead && runePos < len(runes); i++ {
		char := runes[runePos]

		// If closing quote, include it
		if chineseClosingQuotes[char] {
			runePos++
			continue
		}

		// Stop searching on other characters
		break
	}

	// Convert rune index back to byte index and return
	return len(string(runes[:runePos]))
}

// findLastSpaceWithMinLength finds last space position ensuring minLength
func findLastSpaceWithMinLength(text string, minLength int) (string, int) {
	// Safety check: ensure minLength does not exceed text length
	if minLength >= len(text) {
		minLength = len(text) - 1
		if minLength < 0 {
			return "", 0
		}
	}

	// Search for space starting from minLength
	searchText := text[minLength:]
	if idx := strings.LastIndex(searchText, " "); idx != -1 {
		actualIdx := idx + minLength
		return text[:actualIdx], actualIdx
	}
	return "", 0
}

func SplitByPunctuation(text string) []string {
	// Split text using regular expression
	parts := reSplitString.Split(text, -1)

	// Filter out empty strings
	var result []string
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			result = append(result, part)
		}
	}

	return result
}

func RemoveMarkdownSyntax(text string) string {
	// Remove Markdown syntax
	cleaned := reMarkdownChars.ReplaceAllString(text, "")

	return cleaned
}

// RemoveAllPunctuation removes all punctuation marks
func RemoveAllPunctuation(text string) string {
	// Replace punctuation with empty string
	cleaned := reRemoveAllPunctuation.ReplaceAllString(text, "")
	return cleaned
}

// Extract_json_from_string extracts JSON portion from string
func Extract_json_from_string(input string) map[string]interface{} {
	// Extract outermost {}
	start := strings.Index(input, "{")
	if start == -1 {
		fmt.Println("No JSON start delimiter found")
		return nil
	}
	bracketCount := 0
	end := -1
outer:
	for i := start; i < len(input); i++ {
		switch input[i] {
		case '{':
			bracketCount++
		case '}':
			bracketCount--
			if bracketCount == 0 {
				end = i
				break outer
			}
		}
	}
	if end == -1 {
		fmt.Println("No complete JSON structure found")
		return nil
	}
	jsonStr := input[start : end+1]
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &jsonData); err != nil {
		fmt.Println("JSON parsing error:", err)
		return nil
	}
	return jsonData
}

// JoinStrings joins string slice
func JoinStrings(strs []string) string {
	var result string
	for _, s := range strs {
		result += s
	}
	return result
}

// IsWakeUpWord checks if text matches wakeup pattern
func IsWakeUpWord(text string) bool {
	// Check match
	return reWakeUpWord.MatchString(text)
}

// IsInArray checks if text exists in string slice
func IsInArray(text string, array []string) bool {
	for _, item := range array {
		if item == text {
			return true
		}
	}
	return false
}

// RandomSelectFromArray randomly selects an element from slice
func RandomSelectFromArray(array []string) string {
	if len(array) == 0 {
		return "I'm here"
	}

	// Generate random index
	index := rand.Intn(len(array))

	return array[index]
}

func GenerateSecurePassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+[]{}|;:,.<>?/~`"
	password := make([]byte, length)
	for i := range password {
		password[i] = charset[rand.Intn(len(charset))]
	}
	return string(password)
}

// RemoveParentheses removes parentheses and their enclosed content
// Supports Chinese parentheses （） and English parentheses ()
func RemoveParentheses(text string) string {
	// Remove Chinese parentheses and content
	text = reRemoveParenthesesCN.ReplaceAllString(text, "")
	// Remove English parentheses and content
	text = reRemoveParenthesesEN.ReplaceAllString(text, "")
	return text
}

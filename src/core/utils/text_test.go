package utils

import (
	"strings"
	"testing"
)

func TestRemoveAllPunctuation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Test exit with period",
			input:    "Exit.",
			expected: "Exit",
		},
		{
			name:     "CJK punctuation",
			input:    "Hello，world！This is a test。",
			expected: "HelloworldThis is a test",
		},
		{
			name:     "English punctuation",
			input:    "Hello, world! This is a test.",
			expected: "Hello world This is a test",
		},
		{
			name:     "Mixed punctuation",
			input:    "Test: English, Text! Mixed?",
			expected: "Test English Text Mixed",
		},
		{
			name:     "Special symbols",
			input:    "Symbol@#$%^&*()Test",
			expected: "SymbolTest",
		},
		{
			name:     "Quotes and brackets",
			input:    `"Quotes"、'Single'（Parentheses）【Brackets】`,
			expected: "QuotesSingleParenthesesBrackets",
		},
		{
			name:     "Title marks and dashes",
			input:    "《Title》——Author",
			expected: "TitleAuthor",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Punctuation only",
			input:    "！@#$%^&*(),.?;:",
			expected: "",
		},
		{
			name:     "No punctuation",
			input:    "Plain text without punctuation",
			expected: "Plain text without punctuation",
		},
		{
			name:     "Numbers and letters",
			input:    "abc123test!@#",
			expected: "abc123test",
		},
		{
			name:     "Ellipsis and hyphens",
			input:    "Test…Ellipsis—Hyphen-Normal",
			expected: "TestEllipsisHyphenNormal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveAllPunctuation(tt.input)
			if result != tt.expected {
				t.Errorf("RemoveAllPunctuation(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRemoveAllPunctuation_EdgeCases(t *testing.T) {
	t.Run("Spaces only", func(t *testing.T) {
		result := RemoveAllPunctuation("   ")
		expected := "   " // Spaces are not punctuation and should be preserved
		if result != expected {
			t.Errorf("RemoveAllPunctuation(%q) = %q, want %q", "   ", result, expected)
		}
	})

	t.Run("Mixed spaces and punctuation", func(t *testing.T) {
		result := RemoveAllPunctuation("Test , Space！")
		expected := "Test  Space" // Preserve spaces, remove punctuation
		if result != expected {
			t.Errorf("RemoveAllPunctuation(%q) = %q, want %q", "Test , Space！", result, expected)
		}
	})

	t.Run("Consecutive punctuation", func(t *testing.T) {
		result := RemoveAllPunctuation("Test！！！。。。？？？")
		expected := "Test"
		if result != expected {
			t.Errorf("RemoveAllPunctuation(%q) = %q, want %q", "Test！！！。。。？？？", result, expected)
		}
	})
}

// Benchmark tests
func BenchmarkRemoveAllPunctuation(b *testing.B) {
	testString := "This is a test string containing punctuation marks!@#$%^&*()，。？；：\"\"''「」『』（）【】[]{}《》〈〉—–-_~·…‖|\\/*&^%$#@+=<>"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RemoveAllPunctuation(testString)
	}
}

func BenchmarkRemoveAllPunctuation_Short(b *testing.B) {
	testString := "Exit."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RemoveAllPunctuation(testString)
	}
}

func TestSplitAtLastPunctuation_Basic(t *testing.T) {
	text := "Hello, world! This is a test。continue"
	seg, pos := SplitAtLastPunctuation(text)
	expectedIdx := strings.LastIndex(text, "。")
	if expectedIdx == -1 {
		t.Fatalf("setup error: expected to find full stop in %q", text)
	}
	expectedSeg := text[:expectedIdx+len("。")]
	if pos != len(expectedSeg) || seg != expectedSeg {
		t.Fatalf("SplitAtLastPunctuation(%q) = (%q, %d), want (%q, %d)", text, seg, pos, expectedSeg, len(expectedSeg))
	}
}

func TestSplitAtLastPunctuation_DotDecimal_NoSplit(t *testing.T) {
	// contains "3.14" (dot between digits) and total length > 50 so medium punctuations are considered
	text := strings.Repeat("a", 30) + "3.14" + strings.Repeat("b", 20) // length = 54
	seg, pos := SplitAtLastPunctuation(text)
	if seg != "" || pos != 0 {
		t.Fatalf("SplitAtLastPunctuation(%q) = (%q, %d), want (\"\", 0) — decimal point should not cause split", text, seg, pos)
	}
}

func TestSplitAtLastPunctuation_DotBetweenLetters_Split(t *testing.T) {
	// dot between letters should be treated as punctuation when medium punctuations are considered
	text := strings.Repeat("a", 30) + "end.word" + strings.Repeat("b", 20) // length > 50
	seg, pos := SplitAtLastPunctuation(text)
	expectedIdx := strings.LastIndex(text, ".")
	if expectedIdx == -1 {
		t.Fatalf("setup error: expected to find '.' in %q", text)
	}
	expectedSeg := text[:expectedIdx+1]
	if pos != len(expectedSeg) || seg != expectedSeg {
		t.Fatalf("SplitAtLastPunctuation(%q) = (%q, %d), want (%q, %d)", text, seg, pos, expectedSeg, len(expectedSeg))
	}
}

func TestIsWakeUpWord(t *testing.T) {
	if !IsWakeUpWord("hello there") {
		t.Errorf("expected 'hello there' to match wakeup word")
	}
	if !IsWakeUpWord("Hey friend") {
		t.Errorf("expected 'Hey friend' to match wakeup word")
	}
	if IsWakeUpWord("goodbye") {
		t.Errorf("expected 'goodbye' not to match wakeup word")
	}
}

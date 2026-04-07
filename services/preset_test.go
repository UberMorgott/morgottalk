package services

import "testing"

func TestIsHallucination(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		// Empty / whitespace
		{"empty string", "", false},
		{"only spaces", "   ", true},

		// Pure punctuation / noise characters
		{"only dots", "...", true},
		{"ellipsis", "…", true},
		{"musical notes", "♪♫", true},
		{"mixed punctuation", "... ! ? ,", true},
		{"asterisks", "***", true},

		// Known hallucination phrases (Russian)
		{"продолжение следует", "Продолжение следует...", true},
		{"субтитры сделал", "Субтитры сделал DimaTorzworkalov", true},
		{"спасибо за просмотр", "Спасибо за просмотр!", true},
		{"подписывайтесь на канал", "Подписывайтесь на канал", true},
		{"до свидания", "До свидания.", true},
		{"благодарю за внимание", "Благодарю за внимание", true},
		{"редактор субтитров", "Редактор субтитров А.Семкин", true},

		// Known hallucination phrases (English)
		{"thank you", "Thank you.", true},
		{"thanks for watching", "Thanks for watching!", true},
		{"subscribe", "Please subscribe to my channel", true},
		{"like and subscribe", "Like and subscribe!", true},
		{"the end", "The End", true},
		{"to be continued", "To be continued", true},
		{"subtitles by", "Subtitles by the Amara.org community", true},
		{"translated by", "Translated by", true},
		{"you", "You", true},
		{"bye", "Bye.", true},

		// Very short text (<=3 runes after cleaning) → hallucination
		{"single word ok", "Ok", true},  // "Ok" = 2 runes
		{"single word hi", "Hi", true},   // "Hi" = 2 runes
		{"three chars", "Abc", true},     // 3 runes
		{"four chars normal", "Abcd", false}, // 4 runes, not a hallucination phrase

		// Normal transcription text
		{"normal english", "Hello, this is a normal sentence about work", false},
		{"normal russian", "Привет, как дела сегодня?", false},
		{"code snippet", "func main() { fmt.Println(\"hello\") }", false},
		{"longer sentence", "This is a normal transcription of speech that should pass through.", false},
		{"numbers and text", "The meeting is at 3 PM tomorrow", false},
		{"technical text", "We need to refactor the database layer to improve performance", false},

		// Edge cases
		{"unicode text", "日本語のテスト文章です", false},
		{"mixed lang normal", "Давайте обсудим это на meeting завтра", false},
		{"leading trailing whitespace", "\n\t hello world \n", false},
		{"exactly four runes", "test", false},
		{"three runes not in list", "abc", true}, // caught by length check (<=3 runes)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isHallucination(tt.text)
			if got != tt.want {
				t.Errorf("isHallucination(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

// TestIsHallucinationKnownFalsePositives documents known false positives
// caused by strings.Contains matching on short phrases like "you", "thank you".
// These tests assert current (buggy) behavior. When isHallucination is fixed
// to use exact/word-boundary matching, change want to false.
func TestIsHallucinationKnownFalsePositives(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool // current behavior (bug); correct behavior would be false
	}{
		{"contains you substring", "Hello, how are you doing today?", true},
		{"contains thank you in middle", "I want to thank you for helping me", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isHallucination(tt.input); got != tt.want {
				t.Errorf("isHallucination(%q) = %v, want %v (known false positive)", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsEnglishOnlyModel(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
		want      bool
	}{
		{"english base", "base.en", true},
		{"english small", "small.en", true},
		{"english tiny", "tiny.en", true},
		{"english medium", "medium.en", true},
		{"multilingual base", "base", false},
		{"multilingual small", "small", false},
		{"multilingual large", "large-v3", false},
		{"quantized english", "base-q5_1.en", true},
		{"quantized multilingual", "base-q5_1", false},
		{"large v3 turbo", "large-v3-turbo", false},
		{"empty string", "", false},
		{"just .en", ".en", true},
		{"en without dot", "baseen", false},
		{"hyphenated english", "ggml-small.en", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEnglishOnlyModel(tt.modelName)
			if got != tt.want {
				t.Errorf("isEnglishOnlyModel(%q) = %v, want %v", tt.modelName, got, tt.want)
			}
		})
	}
}

package llm

import (
	"strings"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		minTokens int
		maxTokens int
	}{
		{
			name:      "empty string",
			text:      "",
			minTokens: 0,
			maxTokens: 0,
		},
		{
			name:      "single word",
			text:      "hello",
			minTokens: 1,
			maxTokens: 2,
		},
		{
			name:      "simple sentence",
			text:      "The quick brown fox jumps over the lazy dog.",
			minTokens: 8,
			maxTokens: 12,
		},
		{
			name:      "code snippet",
			text:      "func main() {\n\tfmt.Println(\"Hello, World!\")\n}",
			minTokens: 10,
			maxTokens: 20,
		},
		{
			name:      "longer text",
			text:      strings.Repeat("This is a test sentence. ", 100),
			minTokens: 500,
			maxTokens: 700,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.text)
			if got < tt.minTokens || got > tt.maxTokens {
				t.Errorf("EstimateTokens() = %d, want between %d and %d",
					got, tt.minTokens, tt.maxTokens)
			}
		})
	}
}

func TestEstimateTokens_Consistency(t *testing.T) {
	// Same input should always produce same output
	text := "func EstimateTokens(text string) int { return len(text) / 4 }"

	first := EstimateTokens(text)
	for i := 0; i < 10; i++ {
		got := EstimateTokens(text)
		if got != first {
			t.Errorf("EstimateTokens() inconsistent: got %d, want %d", got, first)
		}
	}
}

func TestEstimateTokens_LargeInput(t *testing.T) {
	// Test with a large diff-like input
	largeText := strings.Repeat("+ func foo() error {\n+     return nil\n+ }\n", 1000)

	tokens := EstimateTokens(largeText)

	// Should be roughly proportional to input size
	// 1000 repetitions * ~15 tokens per repetition = ~15000 tokens
	if tokens < 10000 || tokens > 25000 {
		t.Errorf("EstimateTokens() for large input = %d, expected 10000-25000", tokens)
	}
}

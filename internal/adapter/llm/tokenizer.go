// Package llm provides LLM provider adapters.
package llm

import (
	"sync"

	"github.com/pkoukk/tiktoken-go"
)

var (
	defaultEncoder *tiktoken.Tiktoken
	encoderOnce    sync.Once
	encoderErr     error
)

// getEncoder returns the shared tiktoken encoder, initializing it lazily.
// Uses cl100k_base encoding which is used by GPT-4 and is a reasonable
// approximation for other modern LLMs (Claude, Gemini).
func getEncoder() (*tiktoken.Tiktoken, error) {
	encoderOnce.Do(func() {
		defaultEncoder, encoderErr = tiktoken.GetEncoding("cl100k_base")
	})
	return defaultEncoder, encoderErr
}

// EstimateTokens returns an estimated token count for the given text
// using the cl100k_base encoding (GPT-4 tokenizer).
//
// This is suitable for size budgeting across providers since modern LLMs
// use similar tokenization approaches. For exact counts, providers can
// implement their own estimation using native APIs.
func EstimateTokens(text string) int {
	enc, err := getEncoder()
	if err != nil {
		// Fallback to character-based estimate if tiktoken fails
		return len(text) / 4
	}
	tokens := enc.Encode(text, nil, nil)
	return len(tokens)
}

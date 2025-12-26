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
	if text == "" {
		return 0
	}

	enc, err := getEncoder()
	if err != nil {
		// Fallback to conservative character-based estimate if tiktoken fails.
		// Use len/3 (more conservative than len/4) to reduce risk of underestimation.
		// Ensure minimum of 1 for any non-empty text.
		estimate := len(text) / 3
		if estimate < 1 {
			estimate = 1
		}
		return estimate
	}

	// Allow all special tokens to prevent panics on inputs containing
	// sequences like "<|endoftext|>". For estimation purposes, we want
	// to count these as tokens rather than crash.
	tokens := enc.Encode(text, []string{"all"}, nil)
	return len(tokens)
}

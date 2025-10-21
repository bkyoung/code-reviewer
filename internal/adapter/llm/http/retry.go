package http

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// RetryConfig holds configuration for retry logic.
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64
}

// DefaultRetryConfig returns sensible default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 2 * time.Second,
		MaxBackoff:     32 * time.Second,
		Multiplier:     2.0,
	}
}

// ExponentialBackoff calculates wait time with jitter.
// Formula: min(initial * multiplier^attempt, maxBackoff) ± 25% jitter
func ExponentialBackoff(attempt int, config RetryConfig) time.Duration {
	// Calculate base backoff
	backoff := float64(config.InitialBackoff) * math.Pow(config.Multiplier, float64(attempt))

	// Cap at max backoff
	if backoff > float64(config.MaxBackoff) {
		backoff = float64(config.MaxBackoff)
	}

	// Add jitter (±25%)
	jitterRange := 0.25 * backoff
	jitter := (rand.Float64() * 2 * jitterRange) - jitterRange
	result := backoff + jitter

	// Ensure result doesn't exceed max backoff
	if result > float64(config.MaxBackoff) {
		result = float64(config.MaxBackoff)
	}

	// Ensure result is not negative
	if result < 0 {
		result = 0
	}

	return time.Duration(result)
}

// ShouldRetry determines if an error is retryable.
func ShouldRetry(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's our custom Error type
	var httpErr *Error
	if errors.As(err, &httpErr) {
		return httpErr.IsRetryable()
	}

	// Generic errors are not retryable
	return false
}

// Operation is a function that can be retried.
type Operation func(ctx context.Context) error

// RetryWithBackoff executes an operation with exponential backoff retry logic.
func RetryWithBackoff(ctx context.Context, operation Operation, config RetryConfig) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check context before attempting
		if err := ctx.Err(); err != nil {
			return err
		}

		// Execute operation
		err := operation(ctx)
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if error is retryable
		if !ShouldRetry(err) {
			return err // Non-retryable error, fail immediately
		}

		// Max retries exceeded
		if attempt >= config.MaxRetries {
			return err // Return the last error
		}

		// Calculate backoff and wait
		backoff := ExponentialBackoff(attempt, config)

		// Wait with context cancellation support
		select {
		case <-time.After(backoff):
			// Continue to next attempt
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return lastErr
}

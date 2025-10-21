package http

import (
	"sync"
	"time"
)

// Metrics tracks aggregate statistics for API calls.
type Metrics interface {
	// RecordRequest records an API request
	RecordRequest(provider, model string)

	// RecordDuration records request duration
	RecordDuration(provider, model string, duration time.Duration)

	// RecordTokens records token usage
	RecordTokens(provider, model string, tokensIn, tokensOut int)

	// RecordCost records API cost
	RecordCost(provider, model string, cost float64)

	// RecordError records an error
	RecordError(provider, model string, errType ErrorType)

	// GetStats returns current statistics
	GetStats() Stats
}

// Stats contains aggregate statistics.
type Stats struct {
	TotalRequests  int
	TotalTokensIn  int
	TotalTokensOut int
	TotalCost      float64
	TotalDuration  time.Duration
	ErrorCount     int
	ByProvider     map[string]ProviderStats
}

// ProviderStats contains per-provider statistics.
type ProviderStats struct {
	Requests  int
	TokensIn  int
	TokensOut int
	Cost      float64
	Duration  time.Duration
	Errors    int
}

// DefaultMetrics provides in-memory metrics tracking.
type DefaultMetrics struct {
	mu    sync.RWMutex
	stats Stats
}

// NewDefaultMetrics creates a metrics tracker.
func NewDefaultMetrics() *DefaultMetrics {
	return &DefaultMetrics{
		stats: Stats{
			ByProvider: make(map[string]ProviderStats),
		},
	}
}

// RecordRequest increments request counter.
func (m *DefaultMetrics) RecordRequest(provider, model string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.TotalRequests++

	ps := m.stats.ByProvider[provider]
	ps.Requests++
	m.stats.ByProvider[provider] = ps
}

// RecordDuration records API call duration.
func (m *DefaultMetrics) RecordDuration(provider, model string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.TotalDuration += duration

	ps := m.stats.ByProvider[provider]
	ps.Duration += duration
	m.stats.ByProvider[provider] = ps
}

// RecordTokens records token usage.
func (m *DefaultMetrics) RecordTokens(provider, model string, tokensIn, tokensOut int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.TotalTokensIn += tokensIn
	m.stats.TotalTokensOut += tokensOut

	ps := m.stats.ByProvider[provider]
	ps.TokensIn += tokensIn
	ps.TokensOut += tokensOut
	m.stats.ByProvider[provider] = ps
}

// RecordCost records API cost.
func (m *DefaultMetrics) RecordCost(provider, model string, cost float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.TotalCost += cost

	ps := m.stats.ByProvider[provider]
	ps.Cost += cost
	m.stats.ByProvider[provider] = ps
}

// RecordError records an error.
func (m *DefaultMetrics) RecordError(provider, model string, errType ErrorType) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.ErrorCount++

	ps := m.stats.ByProvider[provider]
	ps.Errors++
	m.stats.ByProvider[provider] = ps
}

// GetStats returns a copy of current statistics.
func (m *DefaultMetrics) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Deep copy to avoid race conditions
	statsCopy := Stats{
		TotalRequests:  m.stats.TotalRequests,
		TotalTokensIn:  m.stats.TotalTokensIn,
		TotalTokensOut: m.stats.TotalTokensOut,
		TotalCost:      m.stats.TotalCost,
		TotalDuration:  m.stats.TotalDuration,
		ErrorCount:     m.stats.ErrorCount,
		ByProvider:     make(map[string]ProviderStats),
	}

	for k, v := range m.stats.ByProvider {
		statsCopy.ByProvider[k] = v
	}

	return statsCopy
}

package main

import (
	"context"
	"testing"

	"github.com/bkyoung/code-reviewer/internal/config"
	"github.com/bkyoung/code-reviewer/internal/domain"
	"github.com/bkyoung/code-reviewer/internal/usecase/review"
)

// mockProvider is a simple mock for testing
type mockProvider struct {
	name  string
	model string
}

func (m *mockProvider) Review(ctx context.Context, request review.ProviderRequest) (domain.Review, error) {
	return domain.Review{}, nil
}

func TestCreatePlanningProvider(t *testing.T) {
	tests := []struct {
		name             string
		cfg              *config.Config
		providers        map[string]review.Provider
		obs              observabilityComponents
		wantProvider     bool
		wantProviderType string // "openai", "anthropic", "gemini", "ollama", or "reused"
	}{
		{
			name: "planning enabled with specific OpenAI model - creates dedicated provider",
			cfg: &config.Config{
				Planning: config.PlanningConfig{
					Enabled:  true,
					Provider: "openai",
					Model:    "gpt-4o-mini",
				},
				Providers: map[string]config.ProviderConfig{
					"openai": {
						APIKey: "test-key",
					},
				},
				HTTP: config.HTTPConfig{},
			},
			providers:        map[string]review.Provider{},
			obs:              observabilityComponents{},
			wantProvider:     true,
			wantProviderType: "openai",
		},
		{
			name: "planning enabled with specific Anthropic model - creates dedicated provider",
			cfg: &config.Config{
				Planning: config.PlanningConfig{
					Enabled:  true,
					Provider: "anthropic",
					Model:    "claude-3-haiku-20240307",
				},
				Providers: map[string]config.ProviderConfig{
					"anthropic": {
						APIKey: "test-key",
					},
				},
				HTTP: config.HTTPConfig{},
			},
			providers:        map[string]review.Provider{},
			obs:              observabilityComponents{},
			wantProvider:     true,
			wantProviderType: "anthropic",
		},
		{
			name: "planning enabled with specific Gemini model - creates dedicated provider",
			cfg: &config.Config{
				Planning: config.PlanningConfig{
					Enabled:  true,
					Provider: "gemini",
					Model:    "gemini-2.0-flash-thinking-exp-01-21",
				},
				Providers: map[string]config.ProviderConfig{
					"gemini": {
						APIKey: "test-key",
					},
				},
				HTTP: config.HTTPConfig{},
			},
			providers:        map[string]review.Provider{},
			obs:              observabilityComponents{},
			wantProvider:     true,
			wantProviderType: "gemini",
		},
		{
			name: "planning enabled, no specific model - reuses existing provider",
			cfg: &config.Config{
				Planning: config.PlanningConfig{
					Enabled:  true,
					Provider: "openai",
					Model:    "", // No specific model
				},
				Providers: map[string]config.ProviderConfig{
					"openai": {
						APIKey: "test-key",
					},
				},
				HTTP: config.HTTPConfig{},
			},
			providers: map[string]review.Provider{
				"openai": &mockProvider{name: "openai", model: "gpt-4"},
			},
			obs:              observabilityComponents{},
			wantProvider:     true,
			wantProviderType: "reused",
		},
		{
			name: "planning enabled but provider not found - returns nil",
			cfg: &config.Config{
				Planning: config.PlanningConfig{
					Enabled:  true,
					Provider: "nonexistent",
					Model:    "",
				},
				Providers: map[string]config.ProviderConfig{},
				HTTP:      config.HTTPConfig{},
			},
			providers:        map[string]review.Provider{},
			obs:              observabilityComponents{},
			wantProvider:     false,
			wantProviderType: "",
		},
		{
			name: "planning enabled but API key missing - returns nil",
			cfg: &config.Config{
				Planning: config.PlanningConfig{
					Enabled:  true,
					Provider: "openai",
					Model:    "gpt-4o-mini",
				},
				Providers: map[string]config.ProviderConfig{
					"openai": {
						APIKey: "", // Missing API key
					},
				},
				HTTP: config.HTTPConfig{},
			},
			providers:        map[string]review.Provider{},
			obs:              observabilityComponents{},
			wantProvider:     false,
			wantProviderType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := createPlanningProvider(tt.cfg, tt.providers, tt.obs)

			if tt.wantProvider && got == nil {
				t.Errorf("createPlanningProvider() = nil, want provider")
			}
			if !tt.wantProvider && got != nil {
				t.Errorf("createPlanningProvider() = %v, want nil", got)
			}

			// For reused provider case, verify it's the same instance
			if tt.wantProviderType == "reused" && got != nil {
				expectedProvider := tt.providers[tt.cfg.Planning.Provider]
				if got != expectedProvider {
					t.Errorf("createPlanningProvider() returned different provider instance, want same instance")
				}
			}
		})
	}
}

package http

// Pricing calculates API costs based on token usage.
type Pricing interface {
	// GetCost calculates cost for a given model and token usage
	GetCost(provider, model string, tokensIn, tokensOut int) float64
}

// ModelPricing contains pricing information for a model.
type ModelPricing struct {
	InputPer1M  float64 // Cost per 1M input tokens in USD
	OutputPer1M float64 // Cost per 1M output tokens in USD
}

// DefaultPricing provides cost calculation based on provider pricing.
type DefaultPricing struct {
	prices map[string]map[string]ModelPricing
}

// NewDefaultPricing creates a pricing calculator with current rates.
func NewDefaultPricing() *DefaultPricing {
	return &DefaultPricing{
		prices: buildPricingTable(),
	}
}

// GetCost calculates the cost for a given request.
func (p *DefaultPricing) GetCost(provider, model string, tokensIn, tokensOut int) float64 {
	providerPrices, ok := p.prices[provider]
	if !ok {
		return 0.0
	}

	modelPrice, ok := providerPrices[model]
	if !ok {
		return 0.0
	}

	inputCost := float64(tokensIn) / 1_000_000.0 * modelPrice.InputPer1M
	outputCost := float64(tokensOut) / 1_000_000.0 * modelPrice.OutputPer1M

	return inputCost + outputCost
}

// buildPricingTable returns pricing data for all models.
// Pricing as of: 2025-10-21
// Sources:
// - OpenAI: https://openai.com/api/pricing/
// - Anthropic: https://www.anthropic.com/pricing
// - Gemini: https://ai.google.dev/pricing
// - Ollama: Free (local)
func buildPricingTable() map[string]map[string]ModelPricing {
	return map[string]map[string]ModelPricing{
		"openai": {
			"gpt-4o": {
				InputPer1M:  2.50,
				OutputPer1M: 10.00,
			},
			"gpt-4o-mini": {
				InputPer1M:  0.15,
				OutputPer1M: 0.60,
			},
			"o1": {
				InputPer1M:  15.00,
				OutputPer1M: 60.00,
			},
			"o1-mini": {
				InputPer1M:  3.00,
				OutputPer1M: 12.00,
			},
			"o1-preview": {
				InputPer1M:  15.00,
				OutputPer1M: 60.00,
			},
			"o3-mini": {
				InputPer1M:  1.10,
				OutputPer1M: 4.40,
			},
			"o4-mini": {
				InputPer1M:  1.10,
				OutputPer1M: 4.40,
			},
		},
		"anthropic": {
			"claude-3-5-sonnet-20241022": {
				InputPer1M:  3.00,
				OutputPer1M: 15.00,
			},
			"claude-3-5-sonnet-20240620": {
				InputPer1M:  3.00,
				OutputPer1M: 15.00,
			},
			"claude-3-5-haiku-20241022": {
				InputPer1M:  0.80,
				OutputPer1M: 4.00,
			},
			"claude-3-opus-20240229": {
				InputPer1M:  15.00,
				OutputPer1M: 75.00,
			},
			"claude-3-sonnet-20240229": {
				InputPer1M:  3.00,
				OutputPer1M: 15.00,
			},
			"claude-3-haiku-20240307": {
				InputPer1M:  0.25,
				OutputPer1M: 1.25,
			},
		},
		"gemini": {
			"gemini-1.5-pro": {
				InputPer1M:  1.25,
				OutputPer1M: 5.00,
			},
			"gemini-1.5-flash": {
				InputPer1M:  0.075,
				OutputPer1M: 0.30,
			},
			"gemini-2.0-flash-exp": {
				InputPer1M:  0.00, // Free during preview
				OutputPer1M: 0.00,
			},
		},
		"ollama": {
			// All Ollama models are free (local)
			"codellama": {
				InputPer1M:  0.00,
				OutputPer1M: 0.00,
			},
			"qwen2.5-coder": {
				InputPer1M:  0.00,
				OutputPer1M: 0.00,
			},
			"deepseek-coder": {
				InputPer1M:  0.00,
				OutputPer1M: 0.00,
			},
			"llama3": {
				InputPer1M:  0.00,
				OutputPer1M: 0.00,
			},
			"mistral": {
				InputPer1M:  0.00,
				OutputPer1M: 0.00,
			},
		},
	}
}

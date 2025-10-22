package gemini

// GenerateContentRequest represents a request to Gemini's generateContent API.
type GenerateContentRequest struct {
	Contents          []Content         `json:"contents"`
	SystemInstruction *Content          `json:"systemInstruction,omitempty"`
	GenerationConfig  *GenerationConfig `json:"generationConfig,omitempty"`
	SafetySettings    []SafetySetting   `json:"safetySettings,omitempty"`
}

// Content represents content in the request/response.
type Content struct {
	Parts []Part `json:"parts"`
	Role  string `json:"role,omitempty"` // "user" or "model"
}

// Part represents a part of the content.
type Part struct {
	Text string `json:"text"`
}

// GenerationConfig controls generation parameters.
type GenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	CandidateCount  int     `json:"candidateCount,omitempty"`
}

// SafetySetting configures content filtering.
type SafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// GenerateContentResponse represents a response from Gemini's API.
type GenerateContentResponse struct {
	Candidates    []Candidate   `json:"candidates"`
	UsageMetadata UsageMetadata `json:"usageMetadata"`
}

// Candidate represents a generated candidate response.
type Candidate struct {
	Content      Content `json:"content"`
	FinishReason string  `json:"finishReason"`
}

// UsageMetadata contains token usage information.
type UsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// ErrorResponse represents an error response from Gemini's API.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error information.
type ErrorDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

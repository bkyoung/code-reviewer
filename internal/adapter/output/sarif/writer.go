package sarif

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/brandon/code-reviewer/internal/usecase/review"
)

// Writer implements the review.SARIFWriter interface.
type Writer struct {
	now func() string
}

// NewWriter creates a new SARIF writer.
func NewWriter(now func() string) *Writer {
	return &Writer{now: now}
}

// Write persists a review to disk as a SARIF file.
func (w *Writer) Write(ctx context.Context, artifact review.SARIFArtifact) (string, error) {
	outputDir := filepath.Join(artifact.OutputDir, fmt.Sprintf("%s_%s", artifact.Repository, artifact.TargetRef), w.now())
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	filePath := filepath.Join(outputDir, fmt.Sprintf("review-%s.sarif", artifact.ProviderName))

	sarifDoc := w.convertToSARIF(artifact)

	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create sarif file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(sarifDoc); err != nil {
		return "", fmt.Errorf("failed to encode review to sarif: %w", err)
	}

	return filePath, nil
}

// convertToSARIF converts a domain.Review to SARIF format.
func (w *Writer) convertToSARIF(artifact review.SARIFArtifact) map[string]interface{} {
	results := make([]map[string]interface{}, 0, len(artifact.Review.Findings))

	for _, finding := range artifact.Review.Findings {
		result := map[string]interface{}{
			"ruleId": finding.Category,
			"level":  convertSeverity(finding.Severity),
			"message": map[string]interface{}{
				"text": finding.Description,
			},
			"locations": []map[string]interface{}{
				{
					"physicalLocation": map[string]interface{}{
						"artifactLocation": map[string]interface{}{
							"uri": finding.File,
						},
						"region": map[string]interface{}{
							"startLine": finding.LineStart,
							"endLine":   finding.LineEnd,
						},
					},
				},
			},
		}

		if finding.Suggestion != "" {
			result["fixes"] = []map[string]interface{}{
				{
					"description": map[string]interface{}{
						"text": finding.Suggestion,
					},
				},
			}
		}

		results = append(results, result)
	}

	return map[string]interface{}{
		"version": "2.1.0",
		"$schema": "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		"runs": []map[string]interface{}{
			{
				"tool": map[string]interface{}{
					"driver": map[string]interface{}{
						"name":            artifact.ProviderName,
						"informationUri":  "https://github.com/brandon/code-reviewer",
						"version":         "1.0.0",
						"semanticVersion": "1.0.0",
						"rules": []map[string]interface{}{
							{
								"id":               "code-review",
								"name":             "CodeReview",
								"shortDescription": map[string]interface{}{"text": "AI-powered code review findings"},
								"fullDescription":  map[string]interface{}{"text": "Findings from multi-LLM code review analysis"},
							},
						},
					},
				},
				"results": results,
				"properties": map[string]interface{}{
					"cost":    artifact.Review.Cost,
					"summary": artifact.Review.Summary,
					"model":   artifact.Review.ModelName,
				},
			},
		},
	}
}

// convertSeverity maps our severity levels to SARIF levels.
func convertSeverity(severity string) string {
	switch severity {
	case "critical", "high":
		return "error"
	case "medium":
		return "warning"
	case "low":
		return "note"
	default:
		return "warning"
	}
}

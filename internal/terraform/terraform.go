// Package terraform audits Terraform HCL for security risks.
package terraform

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Champbreed/AegisOps-AI/internal/gemini"
)

// Finding is one issue in the Terraform code.
type Finding struct {
	ResourceType string `json:"resource_type"`
	ResourceName string `json:"resource_name"`
	Finding      string `json:"finding"`
	Severity     string `json:"severity"`
	Remediation  string `json:"remediation"`
}

const systemPrompt = "You are a DevSecOps Expert. Analyze this Terraform HCL for security risks. " +
	"Trace network paths and IAM permissions. Output ONLY a valid JSON array."

var schema = map[string]any{
	"type": "ARRAY",
	"items": map[string]any{
		"type": "OBJECT",
		"properties": map[string]any{
			"resource_type": map[string]any{"type": "STRING"},
			"resource_name": map[string]any{"type": "STRING"},
			"finding":       map[string]any{"type": "STRING"},
			"severity":      map[string]any{"type": "STRING"},
			"remediation":   map[string]any{"type": "STRING"},
		},
	},
}

// Run audits the Terraform file at inPath, writes the findings to outPath,
// and returns them.
func Run(ctx context.Context, c *gemini.Client, inPath, outPath string) ([]Finding, error) {
	code, err := os.ReadFile(inPath)
	if err != nil {
		return nil, fmt.Errorf("terraform: reading input: %w", err)
	}

	text, err := c.Generate(ctx, gemini.Request{
		SystemInstruction: systemPrompt,
		Prompt:            "Audit this Terraform code:\n" + string(code),
		Schema:            schema,
		ThinkingLevel:     "HIGH",
	})
	if err != nil {
		return nil, fmt.Errorf("terraform: %w", err)
	}

	var findings []Finding
	if err := json.Unmarshal([]byte(text), &findings); err != nil {
		return nil, fmt.Errorf("terraform: model returned unexpected JSON: %w", err)
	}

	data, err := json.MarshalIndent(findings, "", "    ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(outPath, append(data, '\n'), 0o644); err != nil {
		return nil, fmt.Errorf("terraform: writing results: %w", err)
	}
	return findings, nil
}

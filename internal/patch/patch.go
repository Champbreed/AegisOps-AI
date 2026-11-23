// Package patch reviews Linux kernel patches for memory-safety issues.
package patch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Champbreed/AegisOps-AI/internal/gemini"
)

// Finding is one reviewed commit.
type Finding struct {
	CommitHash        string   `json:"commit_hash"`
	Title             string   `json:"title"`
	AffectedSubsystem string   `json:"affected_subsystem"`
	SecurityFlag      string   `json:"security_flag"`
	PotentialIssues   []string `json:"potential_issues"`
	Summary           string   `json:"human_readable_summary"`
}

const systemPrompt = `You are a Senior Linux Kernel Security Researcher.
Your task is to perform a deep-dive security audit on kernel patches.
Use your high-reasoning capability to trace memory pointers and detect:
1. Use-After-Free (UAF) conditions in kfree() paths.
2. Race conditions in locked/unlocked sections.
3. Buffer overflows in array indexing.
Return ONLY a valid JSON array.`

var schema = map[string]any{
	"type": "ARRAY",
	"items": map[string]any{
		"type": "OBJECT",
		"properties": map[string]any{
			"commit_hash":            map[string]any{"type": "STRING"},
			"title":                  map[string]any{"type": "STRING"},
			"affected_subsystem":     map[string]any{"type": "STRING"},
			"security_flag":          map[string]any{"type": "STRING", "description": "HIGH, MEDIUM, LOW, or NONE"},
			"potential_issues":       map[string]any{"type": "ARRAY", "items": map[string]any{"type": "STRING"}},
			"human_readable_summary": map[string]any{"type": "STRING"},
		},
	},
}

// Run audits the patches at inPath, writes the findings to outPath, and
// returns them.
func Run(ctx context.Context, c *gemini.Client, inPath, outPath string) ([]Finding, error) {
	patches, err := os.ReadFile(inPath)
	if err != nil {
		return nil, fmt.Errorf("patch: reading patches: %w", err)
	}

	text, err := c.Generate(ctx, gemini.Request{
		SystemInstruction: systemPrompt,
		Prompt:            "Audit these kernel patches and return the JSON array:\n\n" + string(patches),
		Schema:            schema,
		ThinkingLevel:     "HIGH",
	})
	if err != nil {
		return nil, fmt.Errorf("patch: %w", err)
	}

	var findings []Finding
	if err := json.Unmarshal([]byte(text), &findings); err != nil {
		return nil, fmt.Errorf("patch: model returned unexpected JSON: %w", err)
	}

	data, err := json.MarshalIndent(findings, "", "    ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(outPath, append(data, '\n'), 0o644); err != nil {
		return nil, fmt.Errorf("patch: writing results: %w", err)
	}
	return findings, nil
}

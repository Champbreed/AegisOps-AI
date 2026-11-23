// Package finops reviews a Terraform plan for cost and security risks.
package finops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Champbreed/AegisOps-AI/internal/gemini"
)

const systemPrompt = "You are a FinOps and cloud security expert. Output ONLY valid JSON."

// Run reads the plan at planPath, asks Gemini for a cost and security review,
// and writes the pretty-printed JSON report to outPath.
func Run(ctx context.Context, c *gemini.Client, planPath, outPath string) error {
	plan, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("finops: reading plan: %w", err)
	}

	text, err := c.Generate(ctx, gemini.Request{
		SystemInstruction: systemPrompt,
		Prompt:            "Analyze this Terraform Plan for cost and security. Return JSON:\n" + string(plan),
		JSONOutput:        true,
	})
	if err != nil {
		return fmt.Errorf("finops: %w", err)
	}

	// Validate and pretty-print the model's JSON before writing it.
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, []byte(text), "", "    "); err != nil {
		return fmt.Errorf("finops: model returned invalid JSON: %w", err)
	}
	if err := os.WriteFile(outPath, append(pretty.Bytes(), '\n'), 0o644); err != nil {
		return fmt.Errorf("finops: writing report: %w", err)
	}
	fmt.Printf("[FinOps] report written to %s\n", outPath)
	return nil
}

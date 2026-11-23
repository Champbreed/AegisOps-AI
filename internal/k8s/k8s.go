// Package k8s turns a plain-language security requirement into a hardened
// Kubernetes Deployment.
package k8s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Champbreed/AegisOps-AI/internal/gemini"
)

// Result is the model's output.
type Result struct {
	PolicySummary string `json:"policy_summary"`
	HardenedYAML  string `json:"hardened_kubernetes_yaml"`
}

const systemPrompt = "You are a Kubernetes Security Architect specializing in hardening deployments. " +
	"The user will provide a high-level security requirement. You MUST generate two outputs: " +
	"1) A complete, hardened Kubernetes Deployment YAML that satisfies the request. " +
	"2) A summary of the rule implemented, suitable for a policy engine (e.g., OPA or Kube-bench). " +
	"Generate the findings in the exact JSON format specified in the schema."

var schema = map[string]any{
	"type": "OBJECT",
	"properties": map[string]any{
		"policy_summary": map[string]any{
			"type":        "STRING",
			"description": "A concise, single-sentence summary of the security rule that was enforced.",
		},
		"hardened_kubernetes_yaml": map[string]any{
			"type":        "STRING",
			"description": "The complete, hardened Kubernetes Deployment YAML. MUST be a valid YAML string.",
		},
	},
	"required": []string{"policy_summary", "hardened_kubernetes_yaml"},
}

// Run reads the requirement at requestPath, writes the YAML to yamlPath and
// the policy summary to summaryPath, and returns the result.
func Run(ctx context.Context, c *gemini.Client, requestPath, yamlPath, summaryPath string) (Result, error) {
	raw, err := os.ReadFile(requestPath)
	if err != nil {
		return Result{}, fmt.Errorf("k8s: reading request: %w", err)
	}
	requirement := strings.TrimSpace(string(raw))

	text, err := c.Generate(ctx, gemini.Request{
		SystemInstruction: systemPrompt,
		Prompt: "Based on the following security requirement, generate a hardened Kubernetes Deployment YAML " +
			"for a basic Nginx container, and summarize the enforcement rule:\n\n" +
			"SECURITY REQUIREMENT: " + requirement,
		Schema: schema,
	})
	if err != nil {
		return Result{}, fmt.Errorf("k8s: %w", err)
	}

	var result Result
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return Result{}, fmt.Errorf("k8s: model returned unexpected JSON: %w", err)
	}
	yaml := stripCodeFence(result.HardenedYAML)
	if yaml == "" {
		return Result{}, errors.New("k8s: model returned empty YAML")
	}
	result.HardenedYAML = yaml

	if err := os.WriteFile(yamlPath, []byte(yaml+"\n"), 0o644); err != nil {
		return Result{}, fmt.Errorf("k8s: writing YAML: %w", err)
	}
	summary := fmt.Sprintf("Policy Enforced: %s\n\nGenerated for: Nginx-Deployment\n", result.PolicySummary)
	if err := os.WriteFile(summaryPath, []byte(summary), 0o644); err != nil {
		return Result{}, fmt.Errorf("k8s: writing summary: %w", err)
	}
	return result, nil
}

// stripCodeFence removes a Markdown code fence the model may wrap around YAML.
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	lines := strings.Split(s, "\n")
	lines = lines[1:]
	if n := len(lines); n > 0 && strings.HasPrefix(strings.TrimSpace(lines[n-1]), "```") {
		lines = lines[:n-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// Command aegisops runs the AegisOps DevSecOps and FinOps audits and fails
// when a finding is severe enough to block a merge.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Champbreed/AegisOps-AI/internal/env"
	"github.com/Champbreed/AegisOps-AI/internal/finops"
	"github.com/Champbreed/AegisOps-AI/internal/gemini"
	"github.com/Champbreed/AegisOps-AI/internal/k8s"
	"github.com/Champbreed/AegisOps-AI/internal/patch"
	"github.com/Champbreed/AegisOps-AI/internal/terraform"
)

func main() {
	dir := flag.String("dir", ".", "directory containing the audit inputs and an optional .env")
	flag.Parse()

	if err := run(context.Background(), *dir); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println("\nAll audits complete.")
}

// run executes every audit stage. Every stage runs even if an earlier one
// fails, so one report shows all problems. A non-nil error means the
// pipeline should fail.
func run(ctx context.Context, dir string) error {
	if err := env.Load(filepath.Join(dir, ".env")); err != nil {
		return fmt.Errorf("loading .env: %w", err)
	}
	client, err := gemini.New(gemini.Config{APIKey: os.Getenv("GEMINI_API_KEY")})
	if err != nil {
		return fmt.Errorf("set GEMINI_API_KEY in the environment or .env: %w", err)
	}

	p := func(name string) string { return filepath.Join(dir, name) }
	var errs []error

	fmt.Println("=== FinOps & cloud cost audit ===")
	if err := finops.Run(ctx, client, p("test_plan.txt"), p("infrastructure_audit_report.json")); err != nil {
		errs = append(errs, err)
	}

	fmt.Println("\n=== Linux kernel patch audit ===")
	patches, err := patch.Run(ctx, client, p("patches_to_analyze.txt"), p("analysis_results.json"))
	if err != nil {
		errs = append(errs, err)
	}
	for _, f := range patches {
		fmt.Printf("%s [%s] %s\n", f.CommitHash, f.SecurityFlag, f.Summary)
	}

	fmt.Println("\n=== Terraform security audit ===")
	tf, err := terraform.Run(ctx, client, p("test_infra.tf"), p("terraform_audit_results.json"))
	if err != nil {
		errs = append(errs, err)
	}
	for _, f := range tf {
		fmt.Printf("%s.%s [%s] %s\n", f.ResourceType, f.ResourceName, f.Severity, f.Finding)
	}

	fmt.Println("\n=== Kubernetes hardening ===")
	res, err := k8s.Run(ctx, client, p("security_request.txt"),
		p("hardened_deployment.yaml"), p("enforcement_policy_summary.txt"))
	if err != nil {
		errs = append(errs, err)
	} else {
		fmt.Printf("Policy enforced: %s\n", res.PolicySummary)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	if blocked := blockingFindings(patches, tf); len(blocked) > 0 {
		return fmt.Errorf("merge blocked by %d finding(s):\n  %s",
			len(blocked), strings.Join(blocked, "\n  "))
	}
	return nil
}

// blockingFindings lists findings that should stop a merge: kernel patches
// flagged HIGH and Terraform findings rated HIGH or CRITICAL.
func blockingFindings(patches []patch.Finding, tf []terraform.Finding) []string {
	var out []string
	for _, f := range patches {
		if strings.EqualFold(strings.TrimSpace(f.SecurityFlag), "HIGH") {
			out = append(out, fmt.Sprintf("kernel patch %s: %s", f.CommitHash, f.Title))
		}
	}
	for _, f := range tf {
		switch strings.ToUpper(strings.TrimSpace(f.Severity)) {
		case "HIGH", "CRITICAL":
			out = append(out, fmt.Sprintf("terraform %s.%s: %s", f.ResourceType, f.ResourceName, f.Finding))
		}
	}
	return out
}

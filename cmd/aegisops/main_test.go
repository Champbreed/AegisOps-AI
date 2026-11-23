package main

import (
	"testing"

	"github.com/Champbreed/AegisOps-AI/internal/patch"
	"github.com/Champbreed/AegisOps-AI/internal/terraform"
)

func TestBlockingFindings(t *testing.T) {
	patches := []patch.Finding{
		{CommitHash: "a1", Title: "uaf", SecurityFlag: "HIGH"},
		{CommitHash: "b2", Title: "ok", SecurityFlag: "LOW"},
		{CommitHash: "c3", Title: "race", SecurityFlag: " high "},
	}
	tf := []terraform.Finding{
		{ResourceType: "aws_instance", ResourceName: "web", Severity: "CRITICAL", Finding: "open ssh"},
		{ResourceType: "aws_s3_bucket", ResourceName: "logs", Severity: "medium", Finding: "no versioning"},
		{ResourceType: "aws_iam_role", ResourceName: "ci", Severity: "high", Finding: "wildcard"},
	}

	got := blockingFindings(patches, tf)
	if len(got) != 4 {
		t.Fatalf("got %d blocking findings, want 4: %v", len(got), got)
	}
}

func TestBlockingFindingsEmpty(t *testing.T) {
	if got := blockingFindings(nil, nil); len(got) != 0 {
		t.Errorf("expected no blocking findings, got %v", got)
	}
}

package output

import (
	"os"
	"path/filepath"
	"repomedic/internal/rules"
	"strings"
	"testing"
)

func TestMarkdownReportContract(t *testing.T) {
	tmpDir := t.TempDir()
	reportPath := filepath.Join(tmpDir, "repomedic-report.md")

	s, err := NewReportSink(reportPath)
	if err != nil {
		t.Fatalf("NewReportSink failed: %v", err)
	}

	// Provide lifecycle repo info
	repos := []string{"acme/a", "acme/b", "acme/c"}
	for _, r := range repos {
		if err := s.Write(Event{Type: "repo.started", Repo: r}); err != nil {
			t.Fatalf("Write repo.started failed: %v", err)
		}
	}

	// acme/a: Many fails (Top risk)
	s.Write(rules.Result{Repo: "acme/a", RuleID: "default-branch-protected", Status: rules.StatusFail})
	s.Write(rules.Result{Repo: "acme/a", RuleID: "secret-scanning-disabled", Status: rules.StatusFail})

	// acme/b: Blocked by 403
	s.Write(rules.Result{Repo: "acme/b", RuleID: "rule-1", Status: rules.StatusError, Message: "403 Forbidden: Upgrade to GitHub Pro"})
	s.Write(rules.Result{Repo: "acme/b", RuleID: "rule-2", Status: rules.StatusError, Message: "403 Forbidden: feature requires GitHub Pro"})

	// acme/c: Clean
	s.Write(rules.Result{Repo: "acme/c", RuleID: "default-branch-protected", Status: rules.StatusPass})

	if err := s.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	b, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	out := string(b)

	// Required headings
	required := []string{
		"# RepoMedic Scan Report",
		"### 🚨 Executive Risk Brief",
		"### Top Risk Areas",
		"## Controls Failing Across the Fleet",
		"## Top Riskiest Repos",
		"### Monday Morning Hit List",
		"## Overall Risk Posture",
		"## Audit Coverage",
		"## Per-repo status",
		"## Critical findings",
		"## Warnings",
		"## Errors",
		"## Rules evaluated",
	}
	for _, s := range required {
		if !strings.Contains(out, s) {
			t.Fatalf("expected report to contain %q; got:\n%s", s, out)
		}
	}

	// Check Executive Risk Brief content
	if !strings.Contains(out, "repos allow merges to the default branch without required status checks/protection") {
		t.Errorf("expected default branch protection warning in brief")
	}
	if !strings.Contains(out, "repos could not be audited for protections due to plan/permissions (403)") {
		t.Errorf("expected 403 blocker warning in brief")
	}

	// Check Top Risk Areas
	if !strings.Contains(out, "**Default branch allows merges without CI or enforced review gates**") {
		t.Errorf("expected Default Branch Protections in Top Risk Areas")
	}
	if !strings.Contains(out, "can bypass CI/CD & review") {
		t.Errorf("expected impact phrase in Top Risk Areas")
	}
	if !strings.Contains(out, "Baseline = minimum expected controls: branch protection enabled and default-branch merges gated by PRs + required status checks.") {
		t.Errorf("expected Baseline definition in Top Risk Areas")
	}

	// Check Monday Morning Hit List
	if !strings.Contains(out, "| acme/a | Enforce default-branch gates (protection + checks + PR) |") {
		t.Errorf("expected acme/a in Monday Morning Hit List with correct fix")
	}

	// Check Audit Coverage
	if !strings.Contains(out, "**403 Forbidden: feature requires GitHub Pro/Team**: 1 repos (acme/b)") {
		t.Errorf("expected normalized 403 error in audit coverage")
	}
	if !strings.Contains(out, "Blocked coverage creates governance blind spots") {
		t.Errorf("expected audit coverage urgency message")
	}

	// Check Top Riskiest Repos
	if !strings.Contains(out, "| acme/a | 2 | 0 | Default branch unprotected, Secret scanning disabled |") {
		t.Errorf("expected acme/a in top riskiest repos with correct risks")
	}

	// Check Overall Risk Posture
	if !strings.Contains(out, "RepoMedic scanned 3 repositories") {
		t.Errorf("expected overall risk posture summary")
	}
	if !strings.Contains(out, "Immediate priority should be to lock down default branches and resolve audit blockers") {
		t.Errorf("expected overall risk posture priorities")
	}

	// Check Ordering: Overall Risk Posture after Monday Morning Hit List
	mmhlIdx := strings.Index(out, "### Monday Morning Hit List")
	orpIdx := strings.Index(out, "## Overall Risk Posture")
	acIdx := strings.Index(out, "## Audit Coverage")

	if mmhlIdx == -1 || orpIdx == -1 || acIdx == -1 {
		t.Errorf("missing sections for ordering check")
	} else {
		if !(mmhlIdx < orpIdx && orpIdx < acIdx) {
			t.Errorf("expected order: Monday Morning Hit List -> Overall Risk Posture -> Audit Coverage")
		}
	}
}

func TestNormalizeErrorReason(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  foo  bar  ", "foo bar"},
		{"repo.default_branch_protection: something wrong", "something wrong"},
		{"403 Forbidden: Upgrade to GitHub Pro to use this feature", "403 Forbidden: feature requires GitHub Pro/Team"},
		{"403 Forbidden: feature requires GitHub Pro", "403 Forbidden: feature requires GitHub Pro/Team"},
		{"some random error", "some random error"},
	}

	for _, tt := range tests {
		if got := normalizeErrorReason(tt.input); got != tt.expected {
			t.Errorf("normalizeErrorReason(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

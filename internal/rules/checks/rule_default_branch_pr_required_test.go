package checks

import (
	"context"
	"repomedic/internal/rules"
	"repomedic/internal/data"
	"testing"

	"github.com/google/go-github/v66/github"
)

func TestDefaultBranchPRRequiredRule_Evaluate(t *testing.T) {
	rule := &DefaultBranchPRRequiredRule{}
	repo := &github.Repository{
		Owner:         &github.User{Login: github.String("test-org")},
		Name:          github.String("test-repo"),
		FullName:      github.String("test-org/test-repo"),
		DefaultBranch: github.String("main"),
	}

	tests := []struct {
		name           string
		data           map[data.DependencyKey]any
		expectedStatus rules.Status
	}{
		{
			name: "Pass - Classic protection requires PR",
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata:                       repo,
				data.DepRepoDefaultBranchClassicProtection: &github.Protection{RequiredPullRequestReviews: &github.PullRequestReviewsEnforcement{}},
				data.DepRepoDefaultBranchEffectiveRules:    []*github.RepositoryRule{},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "Pass - Effective branch rules require PR",
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata:                       repo,
				data.DepRepoDefaultBranchClassicProtection: nil,
				data.DepRepoDefaultBranchEffectiveRules:    []*github.RepositoryRule{{Type: "pull_request"}},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "Fail - Classic protection exists but does not require PR",
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata:                       repo,
				data.DepRepoDefaultBranchClassicProtection: &github.Protection{},
				data.DepRepoDefaultBranchEffectiveRules:    []*github.RepositoryRule{},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name: "Fail - No classic PR requirement and no effective PR rule",
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata:                       repo,
				data.DepRepoDefaultBranchClassicProtection: nil,
				data.DepRepoDefaultBranchEffectiveRules:    []*github.RepositoryRule{{Type: "required_status_checks"}},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name:           "Error - Dependency missing",
			data:           map[data.DependencyKey]any{},
			expectedStatus: rules.StatusError,
		},
		{
			name: "Error - Invalid dependency type (default branch rules)",
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata:                       repo,
				data.DepRepoDefaultBranchClassicProtection: nil,
				data.DepRepoDefaultBranchEffectiveRules:    "not-a-slice",
			},
			expectedStatus: rules.StatusError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := data.NewMapDataContext(tt.data)
			result, err := rule.Evaluate(context.Background(), repo, dc)
			if err != nil {
				t.Fatalf("Evaluate returned error: %v", err)
			}
			if result.Status != tt.expectedStatus {
				t.Fatalf("expected status %v, got %v", tt.expectedStatus, result.Status)
			}
			if result.Repo != "test-org/test-repo" {
				t.Fatalf("expected repo %s, got %s", "test-org/test-repo", result.Repo)
			}
			if result.RuleID != rule.ID() {
				t.Fatalf("expected rule ID %s, got %s", rule.ID(), result.RuleID)
			}
		})
	}
}

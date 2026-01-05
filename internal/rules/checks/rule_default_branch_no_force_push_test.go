package checks

import (
	"context"
	"repomedic/internal/data"
	"repomedic/internal/rules"
	"testing"

	"github.com/google/go-github/v66/github"
)

func TestDefaultBranchNoForcePushRule_Evaluate(t *testing.T) {
	rule := &DefaultBranchNoForcePushRule{}
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
			name: "Pass - Classic protection blocks force push (AllowForcePushes nil)",
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata:                       repo,
				data.DepRepoDefaultBranchClassicProtection: &github.Protection{},
				data.DepRepoDefaultBranchEffectiveRules:    []*github.RepositoryRule{},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "Pass - Classic protection blocks force push (AllowForcePushes disabled)",
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata:                       repo,
				data.DepRepoDefaultBranchClassicProtection: &github.Protection{AllowForcePushes: &github.AllowForcePushes{Enabled: false}},
				data.DepRepoDefaultBranchEffectiveRules:    []*github.RepositoryRule{},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "Pass - Effective branch rules block force push (non_fast_forward)",
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata:                       repo,
				data.DepRepoDefaultBranchClassicProtection: nil,
				data.DepRepoDefaultBranchEffectiveRules:    []*github.RepositoryRule{{Type: "non_fast_forward"}},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "Pass - Both classic and ruleset block force push",
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata:                       repo,
				data.DepRepoDefaultBranchClassicProtection: &github.Protection{},
				data.DepRepoDefaultBranchEffectiveRules:    []*github.RepositoryRule{{Type: "non_fast_forward"}},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "Fail - Classic protection allows force push",
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata:                       repo,
				data.DepRepoDefaultBranchClassicProtection: &github.Protection{AllowForcePushes: &github.AllowForcePushes{Enabled: true}},
				data.DepRepoDefaultBranchEffectiveRules:    []*github.RepositoryRule{},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name: "Fail - No protection configured",
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata:                       repo,
				data.DepRepoDefaultBranchClassicProtection: nil,
				data.DepRepoDefaultBranchEffectiveRules:    []*github.RepositoryRule{},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name: "Fail - Other ruleset rules but no non_fast_forward",
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata:                       repo,
				data.DepRepoDefaultBranchClassicProtection: nil,
				data.DepRepoDefaultBranchEffectiveRules:    []*github.RepositoryRule{{Type: "pull_request"}, {Type: "required_status_checks"}},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name: "Fail - Classic allows force push even with other rules",
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata:                       repo,
				data.DepRepoDefaultBranchClassicProtection: &github.Protection{AllowForcePushes: &github.AllowForcePushes{Enabled: true}},
				data.DepRepoDefaultBranchEffectiveRules:    []*github.RepositoryRule{{Type: "pull_request"}},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name:           "Error - Dependency missing",
			data:           map[data.DependencyKey]any{},
			expectedStatus: rules.StatusError,
		},
		{
			name: "Error - Invalid dependency type (classic protection)",
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata:                       repo,
				data.DepRepoDefaultBranchClassicProtection: "not-a-protection",
				data.DepRepoDefaultBranchEffectiveRules:    []*github.RepositoryRule{},
			},
			expectedStatus: rules.StatusError,
		},
		{
			name: "Error - Invalid dependency type (effective rules)",
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
				t.Fatalf("expected status %v, got %v (message: %s)", tt.expectedStatus, result.Status, result.Message)
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

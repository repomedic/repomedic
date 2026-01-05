package checks

import (
	"context"
	"repomedic/internal/data"
	"repomedic/internal/rules"
	"testing"

	"github.com/google/go-github/v66/github"
)

func TestDefaultBranchRestrictPushRule_Evaluate(t *testing.T) {
	rule := &DefaultBranchRestrictPushRule{}
	repo := &github.Repository{
		FullName:      github.String("org/repo"),
		DefaultBranch: github.String("main"),
	}

	tests := []struct {
		name           string
		data           map[data.DependencyKey]any
		expectedStatus rules.Status
	}{
		{
			name: "Pass - Classic restrictions exist",
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchClassicProtection: &github.Protection{
					Restrictions: &github.BranchRestrictions{},
				},
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "Pass - Ruleset update restriction exists",
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchClassicProtection: nil,
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{
					{Type: "update"},
				},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "Fail - No restrictions",
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchClassicProtection: &github.Protection{
					Restrictions: nil,
				},
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{
					{Type: "deletion"}, // Not update
				},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name: "Error - Missing dependency",
			data: map[data.DependencyKey]any{
				// Missing DepRepoDefaultBranchClassicProtection
				data.DepRepoDefaultBranchEffectiveRules: []*github.RepositoryRule{},
			},
			expectedStatus: rules.StatusError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := data.NewMapDataContext(tt.data)
			result, err := rule.Evaluate(context.Background(), repo, dc)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if result.Status != tt.expectedStatus {
				t.Errorf("want %v, got %v", tt.expectedStatus, result.Status)
			}
		})
	}
}

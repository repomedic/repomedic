package checks

import (
	"context"
	"repomedic/internal/data"
	"repomedic/internal/rules"
	"testing"

	"github.com/google/go-github/v81/github"
)

func TestDefaultBranchProtectedRule_Evaluate(t *testing.T) {
	rule := &DefaultBranchProtectedRule{}
	repo := &github.Repository{
		FullName:      github.Ptr("org/repo"),
		DefaultBranch: github.Ptr("main"),
	}

	tests := []struct {
		name           string
		data           map[data.DependencyKey]any
		expectedStatus rules.Status
	}{
		{
			name: "Pass - Classic protection exists",
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchClassicProtection: &github.Protection{},
				data.DepRepoDefaultBranchEffectiveRules:    &github.BranchRules{},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "Pass - Ruleset protection exists",
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchClassicProtection: nil,
				data.DepRepoDefaultBranchEffectiveRules: &github.BranchRules{
					Deletion: []*github.BranchRuleMetadata{{}},
				},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "Fail - No protection",
			data: map[data.DependencyKey]any{
				data.DepRepoDefaultBranchClassicProtection: nil,
				data.DepRepoDefaultBranchEffectiveRules:    &github.BranchRules{},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name: "Error - Missing dependency",
			data: map[data.DependencyKey]any{
				// Missing DepRepoDefaultBranchClassicProtection
				data.DepRepoDefaultBranchEffectiveRules: &github.BranchRules{},
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

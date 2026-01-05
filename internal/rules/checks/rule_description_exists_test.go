package checks

import (
	"context"
	"repomedic/internal/rules"
	"repomedic/internal/data"
	"testing"

	"github.com/google/go-github/v66/github"
)

func TestDescriptionExistsRule_Evaluate(t *testing.T) {
	rule := &DescriptionExistsRule{}
	repo := &github.Repository{FullName: github.String("acme/repo")}

	tests := []struct {
		name           string
		data           map[data.DependencyKey]any
		expectedStatus rules.Status
	}{
		{
			name: "Pass - Description Present",
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata: &github.Repository{Description: github.String("desc")},
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "Fail - Description Empty",
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata: &github.Repository{Description: github.String("")},
			},
			expectedStatus: rules.StatusFail,
		},
		{
			name:           "Error - Dependency Missing",
			data:           map[data.DependencyKey]any{},
			expectedStatus: rules.StatusError,
		},
		{
			name: "Error - Unexpected Type",
			data: map[data.DependencyKey]any{
				data.DepRepoMetadata: "not-a-repo",
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

			if tt.expectedStatus != rules.StatusError {
				if result.Repo != "acme/repo" {
					t.Fatalf("expected repo %s, got %s", "acme/repo", result.Repo)
				}
				if result.RuleID != rule.ID() {
					t.Fatalf("expected rule ID %s, got %s", rule.ID(), result.RuleID)
				}
			}
		})
	}
}

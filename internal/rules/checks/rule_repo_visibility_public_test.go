package checks

import (
	"context"
	"testing"

	"repomedic/internal/data"
	"repomedic/internal/rules"

	"github.com/google/go-github/v81/github"
)

func TestRepoVisibilityPublicRule_Evaluate(t *testing.T) {
	tests := []struct {
		name           string
		repo           *github.Repository
		expectedStatus rules.Status
	}{
		{
			name: "private repo passes",
			repo: &github.Repository{
				FullName: github.Ptr("org/private-repo"),
				Private:  github.Ptr(true),
			},
			expectedStatus: rules.StatusPass,
		},
		{
			name: "public repo fails without allow-list",
			repo: &github.Repository{
				FullName:   github.Ptr("org/public-repo"),
				Visibility: github.Ptr("public"),
			},
			expectedStatus: rules.StatusFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &RepoVisibilityPublicRule{}

			// Populate DataContext with repo metadata
			dc := data.NewMapDataContext(map[data.DependencyKey]any{
				data.DepRepoMetadata: tt.repo,
			})

			result, err := rule.Evaluate(context.Background(), tt.repo, dc)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}

			if result.Status != tt.expectedStatus {
				t.Errorf("want %v, got %v (message: %s)", tt.expectedStatus, result.Status, result.Message)
			}
		})
	}
}
